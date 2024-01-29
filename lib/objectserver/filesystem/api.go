package filesystem

import (
	"flag"
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

var (
	objectServerCleanupStartPercent = flag.Int(
		"objectServerCleanupStartPercent", 95, "")
	objectServerCleanupStartSize   flagutil.Size
	objectServerCleanupStopPercent = flag.Int("objectServerCleanupStopPercent",
		90, "")
	objectServerCleanupStopSize flagutil.Size

	// Interface check.
	_ objectserver.FullObjectServer = (*ObjectServer)(nil)
)

func init() {
	flag.Var(&objectServerCleanupStartSize, "objectServerCleanupStartSize", "")
	flag.Var(&objectServerCleanupStopSize, "objectServerCleanupStopSize", "")
}

type objectType struct {
	hash              hash.Hash
	newerUnreferenced *objectType
	olderUnreferenced *objectType
	refcount          uint64
	size              uint64
}

type Config struct {
	BaseDirectory     string
	LockCheckInterval time.Duration
	LockLogTimeout    time.Duration
}

type ObjectServer struct {
	addCallback objectserver.AddCallback
	Config
	gc          objectserver.GarbageCollector
	lockWatcher *lockwatcher.LockWatcher
	Params
	rwLock                sync.RWMutex // Protect the following fields.
	duplicatedBytes       uint64       // Sum of refcount*size for all objects.
	lastGarbageCollection time.Time
	lastMutationTime      time.Time
	objects               map[hash.Hash]*objectType // Only set if object known.
	newestUnreferenced    *objectType
	numDuplicated         uint64 // Sum of refcount for all objects.
	numReferenced         uint64
	numUnreferenced       uint64
	oldestUnreferenced    *objectType
	referencedBytes       uint64
	totalBytes            uint64
	unreferencedBytes     uint64
}

type Params struct {
	Logger log.DebugLogger
}

func NewObjectServer(baseDir string, logger log.Logger) (
	*ObjectServer, error) {
	return newObjectServer(
		Config{BaseDirectory: baseDir},
		Params{Logger: debuglogger.Upgrade(logger)},
	)
}

func NewObjectServerWithConfigAndParams(config Config, params Params) (
	*ObjectServer, error) {
	return newObjectServer(config, params)
}

// AddObject will add an object. Object data are read from reader (length bytes
// are read). The object hash is computed and compared with expectedHash if not
// nil. The following are returned:
//
//	computed hash value
//	a boolean which is true if the object is new
//	an error or nil if no error.
func (objSrv *ObjectServer) AddObject(reader io.Reader, length uint64,
	expectedHash *hash.Hash) (hash.Hash, bool, error) {
	return objSrv.addObject(reader, length, expectedHash)
}

// AdjustRefcounts will increment or decrement the refcounts for each object
// yielded by the specified objects iterator. If there are missing objects or
// the iterator returns an error, the adjustments are reverted and an error is
// returned.
func (objSrv *ObjectServer) AdjustRefcounts(increment bool,
	iterator objectserver.ObjectsIterator) error {
	return objSrv.adjustRefcounts(increment, iterator)
}

func (objSrv *ObjectServer) CheckObjects(hashes []hash.Hash) ([]uint64, error) {
	return objSrv.checkObjects(hashes)
}

// CommitObject will commit (add) a previously stashed object.
func (objSrv *ObjectServer) CommitObject(hashVal hash.Hash) error {
	return objSrv.commitObject(hashVal)
}

func (objSrv *ObjectServer) DeleteObject(hashVal hash.Hash) error {
	return objSrv.deleteObject(hashVal, false)
}

func (objSrv *ObjectServer) DeleteStashedObject(hashVal hash.Hash) error {
	return objSrv.deleteStashedObject(hashVal)
}

// DeleteUnreferenced will delete some or all unreferenced objects.
// The oldest unreferenced objects are deleted first, until both the percentage
// and bytes thresholds are satisfied. The number of bytes and objects deleted
// are returned.
func (objSrv *ObjectServer) DeleteUnreferenced(percentage uint8,
	bytes uint64) (uint64, uint64, error) {
	return objSrv.deleteUnreferenced(percentage, bytes)
}

func (objSrv *ObjectServer) ListUnreferenced() map[hash.Hash]uint64 {
	return objSrv.listUnreferenced()
}

func (objSrv *ObjectServer) SetAddCallback(callback objectserver.AddCallback) {
	objSrv.addCallback = callback
}

// SetGarbageCollector is deprecated.
func (objSrv *ObjectServer) SetGarbageCollector(
	gc objectserver.GarbageCollector) {
	objSrv.gc = gc
}

func (objSrv *ObjectServer) GetObject(hashVal hash.Hash) (
	uint64, io.ReadCloser, error) {
	return objectserver.GetObject(objSrv, hashVal)
}

func (objSrv *ObjectServer) GetObjects(hashes []hash.Hash) (
	objectserver.ObjectsReader, error) {
	return objSrv.getObjects(hashes)
}

func (objSrv *ObjectServer) LastMutationTime() time.Time {
	objSrv.rwLock.RLock()
	defer objSrv.rwLock.RUnlock()
	return objSrv.lastMutationTime
}

func (objSrv *ObjectServer) ListObjectSizes() map[hash.Hash]uint64 {
	return objSrv.listObjectSizes()
}

func (objSrv *ObjectServer) ListObjects() []hash.Hash {
	return objSrv.listObjects()
}

func (objSrv *ObjectServer) NumObjects() uint64 {
	objSrv.rwLock.RLock()
	defer objSrv.rwLock.RUnlock()
	return uint64(len(objSrv.objects))
}

// StashOrVerifyObject will stash an object if it is new or it will verify if it
// already exists. Object data are read from reader (length bytes are read). The
// object hash is computed and compared with expectedHash if not nil.
// The following are returned:
//
//	computed hash value
//	the object data if the object is new, otherwise nil
//	an error or nil if no error.
func (objSrv *ObjectServer) StashOrVerifyObject(reader io.Reader,
	length uint64, expectedHash *hash.Hash) (hash.Hash, []byte, error) {
	return objSrv.stashOrVerifyObject(reader, length, expectedHash)
}

func (objSrv *ObjectServer) WriteHtml(writer io.Writer) {
	objSrv.writeHtml(writer)
}

type ObjectsReader struct {
	objectServer *ObjectServer
	hashes       []hash.Hash
	nextIndex    int64
	sizes        []uint64
}

func (or *ObjectsReader) Close() error {
	return nil
}

func (or *ObjectsReader) NextObject() (uint64, io.ReadCloser, error) {
	return or.nextObject()
}

func (or *ObjectsReader) ObjectSizes() []uint64 {
	return or.sizes
}
