package rpcd

import (
	"io"
	"sync"

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

type rpcType struct {
	subConfiguration          proto.Configuration
	scannerConfiguration      *scanner.Configuration
	fileSystemHistory         *scanner.FileSystemHistory
	objectsDir                string
	rootDir                   string
	networkReaderContext      *rateio.ReaderContext
	netbenchFilename          string
	oldTriggersFilename       string
	rescanObjectCacheFunction func()
	disableScannerFunc        func(disableScanner bool)
	systemGoroutine           *goroutine.Goroutine
	workdirGoroutine          *goroutine.Goroutine
	logger                    log.Logger
	*serverutil.PerUserMethodLimiter
	ownerUsers                   map[string]struct{}
	rwLock                       sync.RWMutex
	getFilesLock                 sync.Mutex
	fetchInProgress              bool // Fetch() & Update() mutually exclusive.
	updateInProgress             bool
	startTimeNanoSeconds         int32 // For Fetch() or Update().
	startTimeSeconds             int64
	lastFetchError               error
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

func Setup(subConfiguration proto.Configuration,
	scannerConfiguration *scanner.Configuration, fsh *scanner.FileSystemHistory,
	objectsDirname string, rootDirname string,
	netReaderContext *rateio.ReaderContext,
	netbenchFname string, oldTriggersFname string,
	disableScannerFunction func(disableScanner bool),
	rescanObjectCacheFunction func(), workdirGoroutine *goroutine.Goroutine,
	logger log.Logger) *HtmlWriter {
	rpcObj := &rpcType{
		subConfiguration:          subConfiguration,
		scannerConfiguration:      scannerConfiguration,
		fileSystemHistory:         fsh,
		objectsDir:                objectsDirname,
		rootDir:                   rootDirname,
		networkReaderContext:      netReaderContext,
		netbenchFilename:          netbenchFname,
		oldTriggersFilename:       oldTriggersFname,
		rescanObjectCacheFunction: rescanObjectCacheFunction,
		disableScannerFunc:        disableScannerFunction,
		systemGoroutine:           goroutine.New(),
		workdirGoroutine:          workdirGoroutine,
		logger:                    logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"Poll": 1,
			}),
	}
	rpcObj.ownerUsers = stringutil.ConvertListToMap(subConfiguration.OwnerUsers,
		false)
	srpc.RegisterNameWithOptions("Subd", rpcObj,
		srpc.ReceiverOptions{
			PublicMethods: []string{
				"Poll",
			}})
	addObjectsHandler := &addObjectsHandlerType{
		objectsDir:           objectsDirname,
		scannerConfiguration: scannerConfiguration,
		logger:               logger,
		rpcObj:               rpcObj,
	}
	srpc.RegisterName("ObjectServer", addObjectsHandler)
	tricorder.RegisterMetric("/image-name", &rpcObj.lastSuccessfulImageName,
		units.None, "name of the image for the last successful update")
	return &HtmlWriter{&rpcObj.lastSuccessfulImageName}
}

func (hw *HtmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}
