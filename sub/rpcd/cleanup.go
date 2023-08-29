package rpcd

import (
	"errors"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func (t *rpcType) Cleanup(conn *srpc.Conn, request sub.CleanupRequest,
	reply *sub.CleanupResponse) error {
	defer t.params.ScannerConfiguration.BoostCpuLimit(t.params.Logger)
	t.params.DisableScannerFunction(true)
	defer t.params.DisableScannerFunction(false)
	t.rwLock.Lock()
	defer t.rwLock.Unlock()
	t.params.Logger.Printf("Cleanup(): %d objects\n", len(request.Hashes))
	if t.fetchInProgress {
		t.params.Logger.Println("Error: fetch in progress")
		return errors.New("fetch in progress")
	}
	if t.updateInProgress {
		t.params.Logger.Println("Error: update progress")
		return errors.New("update in progress")
	}
	for _, hash := range request.Hashes {
		pathname := path.Join(t.config.ObjectsDirectoryName,
			objectcache.HashToFilename(hash))
		var err error
		t.params.WorkdirGoroutine.Run(func() {
			err = fsutil.ForceRemove(pathname)
		})
		if err == nil {
			t.params.Logger.Printf("Deleted: %s\n", pathname)
		} else {
			t.params.Logger.Println(err)
		}
	}
	return nil
}
