package cachingreader

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
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
	flushTimer        *time.Timer
	lruFlushRequestor chan<- chan<- error
	lruUpdateNotifier chan<- struct{}
	params            Params
	rwLock            sync.RWMutex // Protect the following fields.
	data              Stats
	newest            *objectType // For unused objects only.
	objects           map[hash.Hash]*objectType
	oldest            *objectType // For unused objects only.
}

type Params struct {
	BaseDirectory       string
	Logger              log.DebugLogger
	MaximumCachedBytes  uint64               // Default: 1GiB.
	ObjectClient        *client.ObjectClient // Exclusive of ObjectServerAddress
	ObjectServerAddress string               // Exclusive of ObjectClient.
}

type Stats struct {
	CachedBytes      uint64 // Includes LruBytes.
	DownloadingBytes uint64 // Objects being downloaded and cached.
	LruBytes         uint64 // Cached but not in-use objects.

}

func New(params Params) (*ObjectServer, error) {
	return newObjectServer(params)
}

// Deprecated.
func NewObjectServer(baseDir string, maxCachedBytes uint64,
	objectServerAddress string, logger log.DebugLogger) (*ObjectServer, error) {
	return New(Params{
		BaseDirectory:       baseDir,
		Logger:              logger,
		MaximumCachedBytes:  maxCachedBytes,
		ObjectServerAddress: objectServerAddress,
	})
}

func (objSrv *ObjectServer) FetchObjects(hashes []hash.Hash) error {
	return objSrv.fetchObjects(hashes)
}

func (objSrv *ObjectServer) Flush() error {
	return objSrv.flush()
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
