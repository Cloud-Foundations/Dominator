package merge

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/build"
)

type Options struct {
	PrefixPath string
}

type Merger struct {
	builder    *build.Builder
	inodeTable map[string]filesystem.GenericInode // Key: full name.
	options    Options
}

// Merge will return a Merger object which may be used to merge one or more
// FileSystems on top of another.
func New(options *Options) (*Merger, error) {
	return newMerger(options)
}

// GetFileSystem will sort any unsorted directories and return the *FileSystem.
// The Merger should not be used after this.
func (m *Merger) GetFileSystem() *filesystem.FileSystem {
	return m.getFileSystem()
}

// Merge will merge the specified FileSystem on top of another. If there is an
// error such as placing an inode over another inode, an error is returned.
// Placing an empty directory inode over another directory inode with the same
// permissions will not generate an error.
// Placing an inode over another of the same type, data and metadata (igoring
// mtimes) will not generate an error. The mtime of the lower inode will be
// retained (i.e. the mtime of the upper inode is lost).
// If not nil, the options specified will override the Merger options for the
// duration of this call.
func (m *Merger) Merge(fs *filesystem.FileSystem, options *Options) error {
	return m.merge(fs, options)
}
