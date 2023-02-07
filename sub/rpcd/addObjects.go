package rpcd

import (
	"bytes"
	"io"
	"os"
	"path"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/objectserver/rpcd/lib"
)

const (
	dirPerms = syscall.S_IRWXU
)

type objectServer struct {
	baseDir string
	rpcObj  *rpcType
}

func (t *addObjectsHandlerType) AddObjects(conn *srpc.Conn) error {
	defer t.scannerConfiguration.BoostCpuLimit(t.logger)
	objSrv := &objectServer{t.objectsDir, t.rpcObj}
	return lib.AddObjects(conn, conn, conn, objSrv, t.logger)
}

func (objSrv *objectServer) AddObject(reader io.Reader, length uint64,
	expectedHash *hash.Hash) (hash.Hash, bool, error) {
	hashVal, data, err := objectcache.ReadObject(reader, length, expectedHash)
	if err != nil {
		return hashVal, false, err
	}
	filename := path.Join(objSrv.baseDir, objectcache.HashToFilename(hashVal))
	objSrv.rpcObj.workdirGoroutine.Run(func() {
		err = os.MkdirAll(path.Dir(filename), dirPerms)
		if err != nil {
			return
		}
		err = fsutil.CopyToFile(filename, filePerms, bytes.NewReader(data),
			length)
	})
	if err != nil {
		return hashVal, false, err
	}
	return hashVal, true, nil
}
