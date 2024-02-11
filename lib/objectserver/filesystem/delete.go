package filesystem

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

// deleteObject will delete the specified object. If haveLock is false, the
// lock is grabbed. In either case, the lock will be released.
func (objSrv *ObjectServer) deleteObject(hashVal hash.Hash,
	haveLock bool) error {
	var refcount uint64
	if !haveLock {
		objSrv.rwLock.Lock()
	}
	if object := objSrv.objects[hashVal]; object == nil {
		return fmt.Errorf("deleteObject(%x): object unknown", hashVal)
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
	objSrv.rwLock.Unlock()
	if refcount > 0 {
		objSrv.Logger.Printf("deleteObject(%x): refcount: %d\n", refcount)
	}
	filename := path.Join(objSrv.BaseDirectory,
		objectcache.HashToFilename(hashVal))
	return os.Remove(filename)
}
