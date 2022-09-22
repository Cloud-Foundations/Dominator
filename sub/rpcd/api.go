package rpcd

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/rateio"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/scanner"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

const (
	disruptionManagerCancel  = "cancel"
	disruptionManagerCheck   = "check"
	disruptionManagerRequest = "request"
)

type Config struct {
	DisruptionManager        string
	NetworkBenchmarkFilename string
	NoteGeneratorCommand     string
	ObjectsDirectoryName     string
	OldTriggersFilename      string
	RootDirectoryName        string
	SubConfiguration         proto.Configuration
}

type Params struct {
	DisableScannerFunction    func(disableScanner bool)
	FileSystemHistory         *scanner.FileSystemHistory
	Logger                    log.DebugLogger
	NetworkReaderContext      *rateio.ReaderContext
	RescanObjectCacheFunction func()
	ScannerConfiguration      *scanner.Configuration
	SubdDirectory             string
	WorkdirGoroutine          *goroutine.Goroutine
}

type rpcType struct {
	config          Config
	params          Params
	systemGoroutine *goroutine.Goroutine
	*serverutil.PerUserMethodLimiter
	disruptionManagerCommand     chan<- string
	ownerUsers                   map[string]struct{}
	rwLock                       sync.RWMutex // Protect everything below.
	disruptionState              proto.DisruptionState
	getFilesLock                 sync.Mutex
	fetchInProgress              bool // Fetch() & Update() mutually exclusive.
	updateInProgress             bool
	startTimeNanoSeconds         int32 // For Fetch() or Update().
	startTimeSeconds             int64
	lastFetchError               error
	lastNote                     string
	lastSuccessfulImageName      string
	lastUpdateError              error
	lastUpdateHadTriggerFailures bool
	lastWriteError               string
	lockedBy                     *srpc.Conn
	lockedUntil                  time.Time
}

type addObjectsHandlerType struct {
	objectsDir           string
	scannerConfiguration *scanner.Configuration
	logger               log.Logger
	rpcObj               *rpcType
}

type HtmlWriter struct {
	lastNote                *string
	lastSuccessfulImageName *string
}

func Setup(config Config, params Params) *HtmlWriter {
	rpcObj := &rpcType{
		config:                  config,
		params:                  params,
		systemGoroutine:         goroutine.New(),
		lastSuccessfulImageName: readPatchedImageFile(),
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"Poll": 1,
			}),
	}
	rpcObj.startDisruptionManager()
	rpcObj.ownerUsers = stringutil.ConvertListToMap(
		config.SubConfiguration.OwnerUsers, false)
	srpc.RegisterNameWithOptions("Subd", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"GetConfiguration",
				"Poll",
			}})
	addObjectsHandler := &addObjectsHandlerType{
		objectsDir:           config.ObjectsDirectoryName,
		scannerConfiguration: params.ScannerConfiguration,
		logger:               params.Logger,
		rpcObj:               rpcObj,
	}
	srpc.RegisterName("ObjectServer", addObjectsHandler)
	tricorder.RegisterMetric("/image-name", &rpcObj.lastSuccessfulImageName,
		units.None, "name of the image for the last successful update")
	if note, err := rpcObj.generateNote(); err != nil {
		params.Logger.Println(err)
	} else if note != "" {
		rpcObj.lastNote = note
	}
	go rpcObj.startWriteProber()
	return &HtmlWriter{
		lastNote:                &rpcObj.lastNote,
		lastSuccessfulImageName: &rpcObj.lastSuccessfulImageName,
	}
}

func (hw *HtmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}
