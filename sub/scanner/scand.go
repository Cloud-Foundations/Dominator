package scanner

import (
	"runtime"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/html"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

var (
	disableScanRequest chan bool // Must be synchronous (unbuffered).
	enableScanRequest  chan bool // Must be synchronous (unbuffered).
)

func startScannerDaemon(rootDirectoryName string, cacheDirectoryName string,
	configuration *Configuration, logger log.Logger) (
	<-chan *FileSystem, func(disableScanner bool)) {
	fsChannel := make(chan *FileSystem)
	disableScanRequest = make(chan bool)
	enableScanRequest = make(chan bool)
	go scannerDaemon(rootDirectoryName, cacheDirectoryName, configuration,
		fsChannel, logger)
	return fsChannel, doDisableScanner
}

func startScanning(rootDirectoryName string, cacheDirectoryName string,
	configuration *Configuration, logger log.Logger,
	mainFunc func(<-chan *FileSystem, func(disableScanner bool))) {
	html.HandleFunc("/showScanFilter", configuration.showScanFilterHandler)
	fsChannel := make(chan *FileSystem)
	disableScanRequest = make(chan bool)
	enableScanRequest = make(chan bool)
	go mainFunc(fsChannel, doDisableScanner)
	scannerDaemon(rootDirectoryName, cacheDirectoryName, configuration,
		fsChannel, logger)
}

func scannerDaemon(rootDirectoryName string, cacheDirectoryName string,
	configuration *Configuration, fsChannel chan<- *FileSystem,
	logger log.Logger) {
	runtime.LockOSThread()
	loweredPriority := false
	var oldFS FileSystem
	var sleepUntil time.Time
	for ; ; time.Sleep(time.Until(sleepUntil)) {
		sleepUntil = time.Now().Add(time.Second)
		fs, err := scanFileSystem(rootDirectoryName, cacheDirectoryName,
			configuration, &oldFS)
		if err != nil {
			if err == scanner.ErrorScanDisabled {
				continue
			}
			logger.Printf("Error scanning: %s\n", err)
		} else {
			oldFS.InodeTable = fs.InodeTable
			oldFS.DirectoryInode = fs.DirectoryInode
			fsChannel <- fs
			runtime.GC()
			if !loweredPriority {
				syscall.Setpriority(syscall.PRIO_PROCESS, 0, 15)
				loweredPriority = true
			}
			configuration.RestoreCpuLimit(logger)  // Reset after scan.
			configuration.RestoreScanLimit(logger) // Reset after scan.
		}
	}
}

// doDisableScanner will request that scanning be disabled or enabled.
// On disable, the function will block until the scanner has received the
// disable request.
// On enable, the function will block until the scanner has received the enable
// request.
func doDisableScanner(disableScanner bool) {
	if disableScanner {
		disableScanRequest <- true
	} else {
		enableScanRequest <- true
	}
}

// checkScanDisableRequest returns true if there is a pending request to disable
// scanning. The request is consumed. This function does not block if there is
// no disable request, else it will block until scanning is enabled again.
func checkScanDisableRequest() bool {
	select {
	case <-disableScanRequest:
		<-enableScanRequest
		return true
	default:
		return false
	}
}
