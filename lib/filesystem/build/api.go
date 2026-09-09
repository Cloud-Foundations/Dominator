package build

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
)

type Builder struct {
	FileSystem      *filesystem.FileSystem
	inodes          map[filesystem.GenericInode]filesystem.InodeNumber
	nextInodeNumber filesystem.InodeNumber
	sortedMap       map[*filesystem.DirectoryInode]bool
}

func New() *Builder {
	return newBuilder()
}

func (b *Builder) AddDirectoryEntry(parent *filesystem.DirectoryInode,
	leafName string, inode filesystem.GenericInode) error {
	return b.addDirectoryEntry(parent, leafName, inode)
}

func (b *Builder) AddInode(inode filesystem.GenericInode) error {
	return b.addInode(inode)
}

func (b *Builder) Sort() {
	b.sort()
}
