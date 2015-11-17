package filesystem

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"errors"
	"fmt"
	"github.com/Symantec/Dominator/lib/hash"
	"github.com/Symantec/Dominator/lib/objectcache"
	"os"
	"path"
)

const buflen = 65536

func (objSrv *ObjectServer) addObjects(datas [][]byte,
	expectedHashes []*hash.Hash) ([]hash.Hash, error) {
	hashes := make([]hash.Hash, len(datas))
	numAdded := 0
	for index, data := range datas {
		var err error
		var add bool
		hashes[index], add, err = objSrv.addObject(data, expectedHashes[index])
		if err != nil {
			objSrv.logger.Printf("AddObjects(): error: %s", err.Error())
			return nil, err
		}
		if add {
			numAdded++
		}
	}
	objSrv.logger.Printf("AddObjects(): %d of %d are new objects",
		numAdded, len(datas))
	return hashes, nil
}

func (objSrv *ObjectServer) addObject(data []byte, expectedHash *hash.Hash) (
	hash.Hash, bool, error) {
	var hash hash.Hash
	if len(data) < 1 {
		return hash, false, errors.New("zero length object cannot be added")
	}
	hasher := sha512.New()
	if hasher.Size() != len(hash) {
		return hash, false, errors.New("Incompatible hash size")
	}
	if _, err := hasher.Write(data); err != nil {
		return hash, false, err
	}
	copy(hash[:], hasher.Sum(nil))
	if expectedHash != nil {
		if hash != *expectedHash {
			return hash, false, errors.New(fmt.Sprintf(
				"Hash mismatch. Computed=%x, expected=%x", hash, *expectedHash))
		}
	}
	filename := path.Join(objSrv.baseDir, objectcache.HashToFilename(hash))
	// Check for existing object and collision.
	fi, err := os.Lstat(filename)
	if err == nil {
		if !fi.Mode().IsRegular() {
			return hash, false, errors.New("Existing non-file: " + filename)
		}
		if err := collisionCheck(data, filename, fi.Size()); err != nil {
			return hash, false, errors.New("Collision detected: " + err.Error())
		}
		// No collision and no error: it's the same object. Go home early.
		return hash, false, nil
	}
	if err = os.MkdirAll(path.Dir(filename), 0755); err != nil {
		return hash, false, err
	}
	tmpFilename := filename + "~"
	file, err := os.OpenFile(tmpFilename, os.O_CREATE|os.O_WRONLY, 0660)
	if err != nil {
		return hash, false, err
	}
	defer os.Remove(tmpFilename)
	defer file.Close()
	nWritten, err := file.Write(data)
	if err != nil {
		return hash, false, err
	}
	if nWritten != len(data) {
		return hash, false, errors.New(fmt.Sprintf(
			"expected length: %d, got: %d for: %s\n",
			len(data), nWritten, tmpFilename))
	}
	objSrv.sizesMap[hash] = uint64(len(data))
	return hash, true, os.Rename(tmpFilename, filename)
}

func collisionCheck(data []byte, filename string, size int64) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	if int64(len(data)) != size {
		return errors.New(fmt.Sprintf(
			"length mismatch. Data=%d, existing object=%d",
			len(data), size))
	}
	reader := bufio.NewReader(file)
	buffer := make([]byte, 0, buflen)
	for len(data) > 0 {
		numToRead := len(data)
		if numToRead > cap(buffer) {
			numToRead = cap(buffer)
		}
		buf := buffer[:numToRead]
		nread, err := reader.Read(buf)
		if err != nil {
			return err
		}
		if bytes.Compare(data[:nread], buf[:nread]) != 0 {
			return errors.New("content mismatch")
		}
		data = data[nread:]
	}
	return nil
}
