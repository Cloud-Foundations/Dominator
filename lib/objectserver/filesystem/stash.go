package filesystem

import (
	"errors"
	"io"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

var stashDirectory string = ".stash"

func (objSrv *ObjectServer) commitObject(hashVal hash.Hash) error {
	hashName := objectcache.HashToFilename(hashVal)
	filename := path.Join(objSrv.BaseDirectory, hashName)
	stashFilename := path.Join(objSrv.BaseDirectory, stashDirectory, hashName)
	fi, err := os.Lstat(stashFilename)
	if err != nil {
		if length, _ := objSrv.checkObject(hashVal); length > 0 {
			return nil // Previously committed: return success.
		}
		return err
	}
	if !fi.Mode().IsRegular() {
		fsutil.ForceRemove(stashFilename)
		return errors.New("existing non-file: " + stashFilename)
	}
	err = os.MkdirAll(path.Dir(filename), fsutil.PrivateDirPerms)
	if err != nil {
		return err
	}
	objSrv.rwLock.Lock()
	defer objSrv.rwLock.Unlock()
	if _, ok := objSrv.objects[hashVal]; ok {
		fsutil.ForceRemove(stashFilename)
		// Run in a goroutine to keep outside of the lock.
		go objSrv.addCallback(hashVal, uint64(fi.Size()), false)
		return nil
	} else {
		objSrv.add(&objectType{hash: hashVal, size: uint64(fi.Size())})
		if objSrv.addCallback != nil {
			// Run in a goroutine to keep outside of the lock.
			go objSrv.addCallback(hashVal, uint64(fi.Size()), true)
		}
		return os.Rename(stashFilename, filename)
	}
}

func (objSrv *ObjectServer) deleteStashedObject(hashVal hash.Hash) error {
	filename := path.Join(objSrv.BaseDirectory, stashDirectory,
		objectcache.HashToFilename(hashVal))
	return os.Remove(filename)
}

func (objSrv *ObjectServer) stashOrVerifyObject(reader io.Reader,
	length uint64, expectedHash *hash.Hash) (hash.Hash, []byte, error) {
	hashVal, data, err := objectcache.ReadObject(reader, length, expectedHash)
	if err != nil {
		return hashVal, nil, err
	}
	hashName := objectcache.HashToFilename(hashVal)
	filename := path.Join(objSrv.BaseDirectory, hashName)
	// Check for existing object and collision.
	if length, err := objSrv.checkObject(hashVal); err != nil {
		return hashVal, nil, err
	} else if length > 0 {
		if err := collisionCheck(data, filename, int64(length)); err != nil {
			return hashVal, nil, err
		}
		return hashVal, nil, nil
	}
	// Check for existing stashed object and collision.
	stashFilename := path.Join(objSrv.BaseDirectory, stashDirectory, hashName)
	if _, err := objSrv.addOrCompare(hashVal, data, stashFilename); err != nil {
		return hashVal, nil, err
	} else {
		return hashVal, data, nil
	}
}
