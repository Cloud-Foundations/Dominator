package filesystem

import (
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/filesystem/scan"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func newObjectServer(config Config, params Params) (*ObjectServer, error) {
	startTime := time.Now()
	var rusageStart, rusageStop wsyscall.Rusage
	wsyscall.Getrusage(wsyscall.RUSAGE_SELF, &rusageStart)
	sizesMap := make(map[hash.Hash]uint64)
	var mutex sync.Mutex
	err := scan.ScanTree(config.BaseDirectory, func(hashVal hash.Hash,
		size uint64) {
		mutex.Lock()
		sizesMap[hashVal] = size
		mutex.Unlock()
	})
	if err != nil {
		return nil, err
	}
	plural := ""
	if len(sizesMap) != 1 {
		plural = "s"
	}
	err = wsyscall.Getrusage(wsyscall.RUSAGE_SELF, &rusageStop)
	if err != nil {
		params.Logger.Printf("Scanned %d object%s in %s\n",
			len(sizesMap), plural, time.Since(startTime))
	} else {
		userTime := time.Duration(rusageStop.Utime.Sec)*time.Second +
			time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
			time.Duration(rusageStart.Utime.Sec)*time.Second -
			time.Duration(rusageStart.Utime.Usec)*time.Microsecond
		params.Logger.Printf("Scanned %d object%s in %s (%s user CPUtime)\n",
			len(sizesMap), plural, time.Since(startTime), userTime)
	}
	objSrv := &ObjectServer{
		Config:                config,
		Params:                params,
		sizesMap:              sizesMap,
		lastGarbageCollection: time.Now(),
		lastMutationTime:      time.Now(),
	}
	objSrv.lockWatcher = lockwatcher.New(&objSrv.rwLock,
		lockwatcher.LockWatcherOptions{
			CheckInterval: config.LockCheckInterval,
			Logger:        prefixlogger.New("ObjectServer: ", params.Logger),
			LogTimeout:    config.LockLogTimeout,
		})
	return objSrv, nil
}
