package rpcd

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
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

type Config struct {
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
	WorkdirGoroutine          *goroutine.Goroutine
}

type rpcType struct {
	config          Config
	params          Params
	systemGoroutine *goroutine.Goroutine
	*serverutil.PerUserMethodLimiter
	ownerUsers                   map[string]struct{}
	rwLock                       sync.RWMutex
	getFilesLock                 sync.Mutex
	fetchInProgress              bool // Fetch() & Update() mutually exclusive.
	updateInProgress             bool
	startTimeNanoSeconds         int32 // For Fetch() or Update().
	startTimeSeconds             int64
	lastFetchError               error
	lastNote                     string
	lastUpdateError              error
	lastUpdateHadTriggerFailures bool
	lastSuccessfulImageName      string
}

type addObjectsHandlerType struct {
	objectsDir           string
	scannerConfiguration *scanner.Configuration
	logger               log.Logger
	rpcObj               *rpcType
}

type HtmlWriter struct {
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
	rpcObj.ownerUsers = stringutil.ConvertListToMap(
		config.SubConfiguration.OwnerUsers, false)
	srpc.RegisterNameWithOptions("Subd", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
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
	return &HtmlWriter{&rpcObj.lastSuccessfulImageName}
}

func (hw *HtmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

func readPatchedImageFile() string {
	if file, err := os.Open(constants.PatchedImageNameFile); err != nil {
		return ""
	} else {
		defer file.Close()
		var imageName string
		num, err := fmt.Fscanf(file, "%s", &imageName)
		if err == nil && num == 1 {
			return imageName
		}
		return ""
	}
}
