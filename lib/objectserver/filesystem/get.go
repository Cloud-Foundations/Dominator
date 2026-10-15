package filesystem

import (
	"errors"
	"io"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
)

func (objSrv *ObjectServer) getObjects(hashes []hash.Hash) (
	*ObjectsReader, error) {
	objectsReader := ObjectsReader{
		objectServer: objSrv,
		hashes:       hashes,
		nextIndex:    -1,
		sizes:        make([]uint64, 0, len(hashes)),
	}
	pathnames := make([]string, 0, len(hashes))
	for _, hashVal := range hashes {
		size, err := objSrv.checkObject(hashVal)
		if err != nil {
			return nil, err
		}
		if size < 1 {
			hashStr, _ := hashVal.MarshalText()
			return nil, errors.New("missing object: " + string(hashStr))
		}
		pathnames = append(pathnames, path.Join(objSrv.BaseDirectory,
			objectcache.HashToFilename(hashVal)))
		objectsReader.sizes = append(objectsReader.sizes, size)
	}
	objectsReader.filesOpener = fsutil.OpenFiles(nil, pathnames,
		objSrv.FileOpenerQueueSize)
	return &objectsReader, nil
}

func (or *ObjectsReader) close() error {
	or.filesOpener.Cancel()
	return nil
}

func (or *ObjectsReader) nextObject() (uint64, io.ReadCloser, error) {
	or.nextIndex++
	if or.nextIndex >= int64(len(or.hashes)) {
		return 0, nil, errors.New("all objects have been consumed")
	}
	file, err := or.filesOpener.OpenNext()
	if err != nil {
		return 0, nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return 0, nil, err
	}
	return uint64(fi.Size()), file, nil
}
