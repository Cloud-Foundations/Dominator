package cachingreader

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

type objectType struct {
	hash               hash.Hash
	size               uint64
	newer              *objectType
	older              *objectType
	usageCount         uint
	downloadingChannel <-chan struct{} // Closed then set to nil when finished.
}

type downloadingObject struct {
	size uint64
}

type ObjectServer struct {
	baseDir             string
	flushTimer          *time.Timer
	logger              log.DebugLogger
	lruUpdateNotifier   chan<- struct{}
	maxCachedBytes      uint64
	objectServerAddress string
	rwLock              sync.RWMutex // Protect the following fields.
	data                Stats
	newest              *objectType // For unused objects only.
	objects             map[hash.Hash]*objectType
	oldest              *objectType // For unused objects only.
}

type Stats struct {
	CachedBytes      uint64 // Includes LruBytes.
	DownloadingBytes uint64 // Objects being downloaded and cached.
	LruBytes         uint64 // Cached but not in-use objects.

}

func NewObjectServer(baseDir string, maxCachedBytes uint64,
	objectServerAddress string, logger log.DebugLogger) (*ObjectServer, error) {
	return newObjectServer(baseDir, maxCachedBytes, objectServerAddress, logger)
}

func (objSrv *ObjectServer) FetchObjects(hashes []hash.Hash) error {
	return objSrv.fetchObjects(hashes)
}

func (objSrv *ObjectServer) GetObjects(hashes []hash.Hash) (
	objectserver.ObjectsReader, error) {
	return objSrv.getObjects(hashes)
}

func (objSrv *ObjectServer) GetStats() Stats {
	return objSrv.getStats(true)
}

func (objSrv *ObjectServer) LinkObject(filename string,
	hashVal hash.Hash) (bool, error) {
	return objSrv.linkObject(filename, hashVal)
}

func (objSrv *ObjectServer) WriteHtml(writer io.Writer) {
	objSrv.writeHtml(writer)
}
