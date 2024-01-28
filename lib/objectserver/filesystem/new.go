package filesystem

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/filesystem/scan"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func newObjectServer(config Config, params Params) (*ObjectServer, error) {
	objSrv := &ObjectServer{
		Config:                config,
		Params:                params,
		lastGarbageCollection: time.Now(),
		objects:               make(map[hash.Hash]*objectType),
	}
	startTime := time.Now()
	var rusageStart, rusageStop wsyscall.Rusage
	wsyscall.Getrusage(wsyscall.RUSAGE_SELF, &rusageStart)
	err := scan.ScanTree(config.BaseDirectory, func(hashVal hash.Hash,
		size uint64) {
		objSrv.rwLock.Lock()
		objSrv.add(&objectType{hash: hashVal, size: size})
		objSrv.rwLock.Unlock()
	})
	if err != nil {
		return nil, err
	}
	plural := ""
	if len(objSrv.objects) != 1 {
		plural = "s"
	}
	err = wsyscall.Getrusage(wsyscall.RUSAGE_SELF, &rusageStop)
	if err != nil {
		params.Logger.Printf("Scanned %d object%s in %s\n",
			len(objSrv.objects), plural, time.Since(startTime))
	} else {
		userTime := time.Duration(rusageStop.Utime.Sec)*time.Second +
			time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
			time.Duration(rusageStart.Utime.Sec)*time.Second -
			time.Duration(rusageStart.Utime.Usec)*time.Microsecond
		params.Logger.Printf("Scanned %d object%s in %s (%s user CPUtime)\n",
			len(objSrv.objects), plural, time.Since(startTime), userTime)
	}
	go objSrv.garbageCollectorLoop()
	objSrv.lockWatcher = lockwatcher.New(&objSrv.rwLock,
		lockwatcher.LockWatcherOptions{
			CheckInterval: config.LockCheckInterval,
			Logger:        prefixlogger.New("ObjectServer: ", params.Logger),
			LogTimeout:    config.LockLogTimeout,
		})
	return objSrv, nil
}
