package scanner

import (
	"io"
	"os"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/cpulimiter"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/fsrateio"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type Hasher interface {
	Hash(reader io.Reader, length uint64) (hash.Hash, error)
}

// Secret Imaginator business.
type readingHasher interface {
	Hasher
	ReadAndHash(inode *filesystem.RegularInode, file *os.File,
		stat *wsyscall.Stat_t) (bool, error)
}

type simpleHasher bool // If true, ignore short reads.

type cpuLimitedHasher struct {
	limiter *cpulimiter.CpuLimiter
	hasher  Hasher
}

type FileSystem struct {
	params      Params
	dev         uint64
	inodeNumber uint64
	fsLock      sync.Locker // Protect everything below.
	filesystem.FileSystem
	hashWaiters map[uint64]<-chan struct{} // Key: inode number.
}

type Params struct {
	FsScanContext           *fsrateio.ReaderContext
	RootDirectoryName       string
	Runner                  concurrent.MeasuringRunner
	ScanFilter              *filter.Filter
	CheckScanDisableRequest func() bool
	Hasher                  Hasher
	OldFS                   *FileSystem
}

func MakeRegularInode(stat *wsyscall.Stat_t) *filesystem.RegularInode {
	return makeRegularInode(stat)
}

func MakeSymlinkInode(stat *wsyscall.Stat_t) *filesystem.SymlinkInode {
	return makeSymlinkInode(stat)
}

func MakeSpecialInode(stat *wsyscall.Stat_t) *filesystem.SpecialInode {
	return makeSpecialInode(stat)
}

func ScanFileSystem(rootDirectoryName string,
	fsScanContext *fsrateio.ReaderContext, scanFilter *filter.Filter,
	checkScanDisableRequest func() bool, hasher Hasher, oldFS *FileSystem) (
	*FileSystem, error) {
	return scanFileSystem(Params{
		FsScanContext:           fsScanContext,
		RootDirectoryName:       rootDirectoryName,
		ScanFilter:              scanFilter,
		CheckScanDisableRequest: checkScanDisableRequest,
		Hasher:                  hasher,
		OldFS:                   oldFS,
	})
}

func ScanFileSystemWithParams(params Params) (*FileSystem, error) {
	return scanFileSystem(params)
}

func (fs *FileSystem) GetObject(hashVal hash.Hash) (
	uint64, io.ReadCloser, error) {
	return fs.getObject(hashVal)
}

func GetSimpleHasher(ignoreShortReads bool) Hasher {
	return simpleHasher(ignoreShortReads)
}

func (h simpleHasher) Hash(reader io.Reader, length uint64) (hash.Hash, error) {
	return h.hash(reader, length)
}

func NewCpuLimitedHasher(cpuLimiter *cpulimiter.CpuLimiter,
	hasher Hasher) cpuLimitedHasher {
	return cpuLimitedHasher{cpuLimiter, hasher}
}

func (h cpuLimitedHasher) Hash(reader io.Reader, length uint64) (
	hash.Hash, error) {
	return h.hash(reader, length)
}
