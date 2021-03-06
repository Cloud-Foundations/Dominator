package main

import (
	"crypto/sha512"
	"io"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

func convertToObject(pathname, objectsDir string) error {
	file, err := os.Open(pathname)
	if err != nil {
		return err
	}
	defer file.Close()
	hasher := sha512.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return err
	}
	var hashVal hash.Hash
	copy(hashVal[:], hasher.Sum(nil))
	objPathname := path.Join(objectsDir, objectcache.HashToFilename(hashVal))
	if err = os.MkdirAll(path.Dir(objPathname), 0755); err != nil {
		return err
	}
	return fsutil.ForceRename(pathname, objPathname)
}
