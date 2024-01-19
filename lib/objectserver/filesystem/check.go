package filesystem

import (
	"fmt"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

func (objSrv *ObjectServer) checkObjects(hashes []hash.Hash) ([]uint64, error) {
	sizesList := make([]uint64, len(hashes))
	for index, hash := range hashes {
		var err error
		sizesList[index], err = objSrv.checkObject(hash)
		if err != nil {
			return nil, err
		}
	}
	return sizesList, nil
}

func (objSrv *ObjectServer) checkObject(hashVal hash.Hash) (uint64, error) {
	objSrv.rwLock.RLock()
	object, ok := objSrv.objects[hashVal]
	objSrv.rwLock.RUnlock()
	if ok {
		return object.size, nil
	}
	filename := path.Join(objSrv.BaseDirectory,
		objectcache.HashToFilename(hashVal))
	fi, err := os.Lstat(filename)
	if err != nil {
		return 0, nil
	}
	if fi.Mode().IsRegular() {
		if fi.Size() < 1 {
			return 0, fmt.Errorf("zero length file: %s", filename)
		}
		return uint64(fi.Size()), nil
	}
	return 0, nil
}
