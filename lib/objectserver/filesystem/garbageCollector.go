package filesystem

import (
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

func sanitisePercentage(percent int) uint64 {
	if percent < 1 {
		return 1
	}
	if percent > 99 {
		return 99
	}
	return uint64(percent)
}

func (objSrv *ObjectServer) garbageCollector() (uint64, error) {
	objSrv.rwLock.Lock()
	if time.Since(objSrv.lastGarbageCollection) < time.Second {
		objSrv.rwLock.Unlock()
		return 0, nil
	}
	objSrv.lastGarbageCollection = time.Now()
	var bytesToDelete uint64
	if objectServerCleanupStopSize < objectServerCleanupStartSize &&
		objSrv.unreferencedBytes > uint64(objectServerCleanupStartSize) {
		bytesToDelete = objSrv.unreferencedBytes -
			uint64(objectServerCleanupStopSize)
	}
	objSrv.rwLock.Unlock()
	if free, capacity, err := objSrv.getSpaceMetrics(); err != nil {
		objSrv.Logger.Println(err)
	} else {
		cleanupStartPercent := sanitisePercentage(
			*objectServerCleanupStartPercent)
		cleanupStopPercent := sanitisePercentage(
			*objectServerCleanupStopPercent)
		if cleanupStopPercent >= cleanupStartPercent {
			cleanupStopPercent = cleanupStartPercent - 1
		}
		utilisation := (capacity - free) * 100 / capacity
		if utilisation >= cleanupStartPercent {
			relativeBytesToDelete := (utilisation - cleanupStopPercent) *
				capacity / 100
			if relativeBytesToDelete > bytesToDelete {
				bytesToDelete = relativeBytesToDelete
			}
		}
	}
	if bytesToDelete < 1 {
		return 0, nil
	}
	var bytesDeleted uint64
	var err error
	if objSrv.gc == nil {
		bytesDeleted, _, err = objSrv.deleteUnreferenced(0, bytesToDelete)
	} else {
		bytesDeleted, err = objSrv.gc(bytesToDelete)
	}
	if err != nil {
		objSrv.Logger.Printf(
			"Error collecting garbage, only deleted: %s of %s: %s\n",
			format.FormatBytes(bytesDeleted), format.FormatBytes(bytesToDelete),
			err)
		return 0, err
	}
	return bytesDeleted, nil
}

// garbageCollectorLoop will periodically delete unreferenced objects if space
// is running low. It returns if an external (deprecated) garbage collector is
// set.
func (objSrv *ObjectServer) garbageCollectorLoop() {
	for time.Sleep(5 * time.Second); objSrv.gc == nil; time.Sleep(time.Second) {
		objSrv.garbageCollector()
	}
}

// getSpaceMetrics returns freeSpace, capacity.
func (t *ObjectServer) getSpaceMetrics() (uint64, uint64, error) {
	fd, err := syscall.Open(t.BaseDirectory, syscall.O_RDONLY, 0)
	if err != nil {
		t.Logger.Printf("error opening: %s: %s", t.BaseDirectory, err)
		return 0, 0, err
	} else {
		defer syscall.Close(fd)
		var statbuf syscall.Statfs_t
		if err := syscall.Fstatfs(fd, &statbuf); err != nil {
			t.Logger.Printf("error getting file-system stats: %s\n", err)
			return 0, 0, err
		}
		rootReservation := statbuf.Bfree - statbuf.Bavail
		return uint64(statbuf.Bavail) * uint64(statbuf.Bsize),
			uint64(statbuf.Blocks-rootReservation) * uint64(statbuf.Bsize), nil
	}
}
