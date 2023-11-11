package filesystem

import (
	"os"
	"path"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

func (objSrv *ObjectServer) deleteObject(hashVal hash.Hash) error {
	filename := path.Join(objSrv.BaseDirectory,
		objectcache.HashToFilename(hashVal))
	if err := os.Remove(filename); err != nil {
		return err
	}
	objSrv.rwLock.Lock()
	delete(objSrv.sizesMap, hashVal)
	objSrv.lastMutationTime = time.Now()
	objSrv.rwLock.Unlock()
	return nil
}
