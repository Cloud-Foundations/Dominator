package scanner

import (
	"io"

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

type openingHasher interface {
	Hasher
	OpenAndHash(inode *filesystem.RegularInode, pathName string) (bool, error)
}

type simpleHasher bool // If true, ignore short reads.

type cpuLimitedHasher struct {
	limiter *cpulimiter.CpuLimiter
	hasher  Hasher
}

type FileSystem struct {
	rootDirectoryName       string
	fsScanContext           *fsrateio.ReaderContext
	scanFilter              *filter.Filter
	checkScanDisableRequest func() bool
	hasher                  Hasher
	dev                     uint64
	inodeNumber             uint64
	filesystem.FileSystem
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
	return scanFileSystem(rootDirectoryName, fsScanContext, scanFilter,
		checkScanDisableRequest, hasher, oldFS)
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
