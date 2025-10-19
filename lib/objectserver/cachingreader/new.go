package cachingreader

import (
	"errors"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/filesystem/scan"
)

func newObjectServer(params Params) (*ObjectServer, error) {
	if params.MaximumCachedBytes < 1 {
		params.MaximumCachedBytes = 1 << 30
	}
	if params.ObjectClient != nil && params.ObjectServerAddress != "" {
		return nil, errors.New("cannot specify object client and address")
	}
	startTime := time.Now()
	var rusageStart, rusageStop syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStart)
	var mutex sync.Mutex
	objects := make(map[hash.Hash]*objectType)
	var cachedBytes uint64
	err := scan.ScanTree(params.BaseDirectory,
		func(hashVal hash.Hash, size uint64) {
			mutex.Lock()
			cachedBytes += size
			objects[hashVal] = &objectType{hash: hashVal, size: size}
			mutex.Unlock()
		})
	if err != nil {
		return nil, err
	}
	plural := ""
	if len(objects) != 1 {
		plural = "s"
	}
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusageStop)
	userTime := time.Duration(rusageStop.Utime.Sec)*time.Second +
		time.Duration(rusageStop.Utime.Usec)*time.Microsecond -
		time.Duration(rusageStart.Utime.Sec)*time.Second -
		time.Duration(rusageStart.Utime.Usec)*time.Microsecond
	params.Logger.Printf("Scanned %d object%s in %s (%s user CPUtime)\n",
		len(objects), plural, time.Since(startTime), userTime)
	lruFlushRequestor := make(chan chan<- error, 1)
	lruUpdateNotifier := make(chan struct{}, 1)
	objSrv := &ObjectServer{
		flushTimer:        time.NewTimer(time.Minute),
		lruFlushRequestor: lruFlushRequestor,
		lruUpdateNotifier: lruUpdateNotifier,
		params:            params,
		data:              Stats{CachedBytes: cachedBytes},
		objects:           objects,
	}
	objSrv.flushTimer.Stop()
	if err := objSrv.loadLru(); err != nil {
		return nil, err
	}
	objSrv.linkOrphanedEntries()
	go objSrv.flusher(lruFlushRequestor, lruUpdateNotifier)
	return objSrv, nil
}
