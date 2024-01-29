package filesystem

import (
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

func (objSrv *ObjectServer) deleteObject(hashVal hash.Hash,
	haveLock bool) error {
	filename := path.Join(objSrv.BaseDirectory,
		objectcache.HashToFilename(hashVal))
	if err := os.Remove(filename); err != nil {
		return err
	}
	var refcount uint64
	if !haveLock {
		objSrv.rwLock.Lock()
	}
	if object := objSrv.objects[hashVal]; object == nil {
		objSrv.Logger.Printf("deleteObject(%x): object does not exist", hashVal)
	} else {
		refcount = object.refcount
		delete(objSrv.objects, hashVal)
		objSrv.duplicatedBytes -= object.size * object.refcount
		objSrv.lastMutationTime = time.Now()
		objSrv.numDuplicated -= object.refcount
		if object.refcount > 0 {
			objSrv.numReferenced--
			objSrv.referencedBytes -= object.size
		}
		objSrv.removeUnreferenced(object)
		objSrv.totalBytes -= object.size
	}
	if !haveLock {
		objSrv.rwLock.Unlock()
	}
	if refcount > 0 {
		objSrv.Logger.Printf("deleteObject(%x): refcount: %d\n", refcount)
	}
	return nil
}
