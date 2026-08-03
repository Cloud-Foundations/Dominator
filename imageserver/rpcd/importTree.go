package rpcd

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/tree"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fstree"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

const (
	maxConcurrency = 128
)

type combinedGetter struct {
	logger         log.DebugLogger
	objSrv         objectserver.ObjectServer
	rawGetter      fstree.Getter
	mutex          sync.Mutex // Protect everything below.
	bytesFetched   uint64
	objectsFetched uint64
}

func getObject(objSrv objectserver.ObjectServer, hashVal hash.Hash) (
	uint64, io.ReadCloser, error) {
	sizes, err := objSrv.CheckObjects([]hash.Hash{hashVal})
	if err != nil {
		return 0, nil, err
	}
	if sizes[0] < 1 {
		return 0, nil, nil
	}
	size, rc, err := objSrv.GetObject(hashVal)
	return size, rc, err
}

func (t *srpcType) ImportTree(conn *srpc.Conn,
	request imageserver.ImportTreeRequest,
	reply *imageserver.ImportTreeResponse) error {
	if err := t.importTree(conn, request, reply); err != nil {
		reply.Error = errors.ErrorToString(err)
	}
	return nil
}

func (t *srpcType) importTree(conn *srpc.Conn,
	request imageserver.ImportTreeRequest,
	reply *imageserver.ImportTreeResponse) error {
	if err := t.checkMutability(); err != nil {
		return err
	}
	baseUrl, treeHash, err := fstree.SplitTreeUrl(request.TreeUrl)
	if err != nil {
		return err
	}
	directoryExists := t.imageDataBase.CheckDirectory(request.DirectoryName)
	if !directoryExists {
		return fmt.Errorf("directory: %s does not exist", request.DirectoryName)
	}
	imageName := filepath.Join(request.DirectoryName, treeHash)
	imageExists := t.imageDataBase.CheckImage(imageName)
	if imageExists {
		return fmt.Errorf("image: %s already exists", imageName)
	}
	username := conn.Username()
	var logPrefix string
	if username == "" {
		logPrefix = fmt.Sprintf("ImportTree()")
	} else {
		logPrefix = fmt.Sprintf("ImportTree(%s)", username)
	}
	t.logger.Printf("%s: tree: %s\n", logPrefix, request.TreeUrl)
	rawGetter, err := fstree.NewGetter(fstree.GetterParams{
		BaseUrl:     baseUrl,
		IoSemaphore: make(chan struct{}, maxConcurrency),
		Logger:      t.logger,
	})
	if err != nil {
		return err
	}
	getter := &combinedGetter{
		logger:    t.logger,
		objSrv:    t.objSrv,
		rawGetter: rawGetter,
	}
	// Walk the tree, importing symlink and tree objects along the way.
	startTime := time.Now()
	fileSystem, hashes, objectSizes, err := tree.Get(getter, request.TreeUrl,
		t.logger)
	if err != nil {
		t.logger.Printf("%s: error walking tree: %s", logPrefix, err)
		return err
	}
	reply.TreeBytesAdded = getter.bytesFetched
	reply.TreesDownloadTime = time.Since(startTime)
	reply.TreeObjectsAdded = getter.objectsFetched
	speed := float64(reply.TreeBytesAdded) / reply.TreesDownloadTime.Seconds()
	t.logger.Printf(
		"Imported %d (%s) new tree objects in: %s (%s/s)\n",
		reply.TreeObjectsAdded,
		format.FormatBytes(reply.TreeBytesAdded),
		format.Duration(reply.TreesDownloadTime),
		format.FormatBytes(uint64(speed)))
	// Import missing objects.
	getter.bytesFetched = 0
	getter.objectsFetched = 0
	startTime = time.Now()
	if err := getter.importObjects(hashes, objectSizes); err != nil {
		t.logger.Printf("%s: error importing objects: %s", logPrefix, err)
		return err
	}
	reply.FileBytesAdded = getter.bytesFetched
	reply.FileObjectsAdded = getter.objectsFetched
	reply.FilesDownloadTime = time.Since(startTime)
	speed = float64(reply.FileBytesAdded) / reply.FilesDownloadTime.Seconds()
	t.logger.Printf(
		"Imported %d (%s) new file objects in: %s (%s/s)\n",
		reply.FileObjectsAdded,
		format.FormatBytes(reply.FileBytesAdded),
		format.Duration(reply.FilesDownloadTime),
		format.FormatBytes(uint64(speed)))
	img := &image.Image{
		CreatedBy:  username,   // Must always set this field.
		CreatedOn:  time.Now(), // Must always set this field.
		ExpiresAt:  request.ExpiresAt,
		FileSystem: fileSystem,
	}
	img.FileSystem.RebuildInodePointers()
	err = t.imageDataBase.AddImage(img, imageName, conn.GetAuthInformation())
	if err != nil {
		t.logger.Printf("%s: error adding image: %s", logPrefix, err)
		return err
	}
	reply.ImageName = imageName
	t.logger.Printf("%s: image: %s\n", logPrefix, imageName)
	return nil
}

func (g *combinedGetter) GetBlobData(hashVal hash.Hash) ([]byte, error) {
	rc, _, err := g.GetBlobReader(hashVal)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (g *combinedGetter) GetBlobReader(hashVal hash.Hash) (
	io.ReadCloser, uint64, error) {
	return g.getDataReader(hashVal, false)
}

func (g *combinedGetter) GetTree(hashVal hash.Hash) (*fstree.Tree, error) {
	rc, _, err := g.GetTreeReader(hashVal)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return fstree.DecodeTree(rc)
}

func (g *combinedGetter) GetTreeReader(hashVal hash.Hash) (
	io.ReadCloser, uint64, error) {
	return g.getDataReader(hashVal, true)
}

func (g *combinedGetter) getDataReader(hashVal hash.Hash, isTree bool) (
	io.ReadCloser, uint64, error) {
	size, rc, err := getObject(g.objSrv, hashVal)
	if err != nil {
		return nil, 0, err
	}
	if size > 0 {
		return rc, size, nil
	}
	if isTree {
		rc, size, err = g.rawGetter.GetTreeReader(hashVal)
	} else {
		rc, size, err = g.rawGetter.GetBlobReader(hashVal)
	}
	if err != nil {
		return nil, 0, err
	}
	computedHash, added, err := g.objSrv.AddObject(rc, size, &hashVal)
	if err != nil {
		rc.Close()
		return nil, 0, err
	}
	rc.Close()
	if computedHash != hashVal {
		return nil, 0, fmt.Errorf("claimed hash: %x != computed hash: %",
			hashVal, computedHash)
	}
	if added {
		g.mutex.Lock()
		g.bytesFetched += size
		g.objectsFetched++
		g.mutex.Unlock()
	}
	size, rc, err = g.objSrv.GetObject(hashVal)
	return rc, size, nil
}

func (g *combinedGetter) importObjects(hashes []hash.Hash,
	sizes []uint64) error {
	foundSizes, err := g.objSrv.CheckObjects(hashes)
	if err != nil {
		return err
	}
	runner := concurrent.NewState(maxConcurrency)
	for index, foundSize := range foundSizes {
		hashVal := hashes[index]
		if foundSize != 0 {
			if foundSize != sizes[index] {
				return fmt.Errorf("existing size: %d != %d for: %x",
					foundSize, sizes[index], hashVal)
			}
			continue
		}
		err := runner.GoRun(func() error {
			return g.importObject(hashVal)
		})
		if err != nil {
			return err
		}
	}
	return runner.Reap()
}

func (g *combinedGetter) importObject(hashVal hash.Hash) error {
	rc, size, err := g.rawGetter.GetBlobReader(hashVal)
	if err != nil {
		return err
	}
	defer rc.Close()
	computedHash, added, err := g.objSrv.AddObject(rc, size, &hashVal)
	if err != nil {
		return err
	}
	if computedHash != hashVal {
		return fmt.Errorf("claimed hash: %x != computed hash: %",
			hashVal, computedHash)
	}
	if added {
		g.mutex.Lock()
		g.bytesFetched += size
		g.objectsFetched++
		g.mutex.Unlock()
	} else {
		g.logger.Printf("imported object already exists: %x\n", hashVal)
	}
	return nil
}
