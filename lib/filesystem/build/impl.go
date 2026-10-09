package build

import (
	"fmt"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func newBuilder() *Builder {
	fs := &filesystem.FileSystem{}
	fs.DirectoryInode.Mode = wsyscall.S_IFDIR | wsyscall.S_IRWXU |
		wsyscall.S_IRGRP | wsyscall.S_IXGRP | wsyscall.S_IROTH |
		wsyscall.S_IXOTH
	fs.InodeTable = make(filesystem.InodeTable)
	b := &Builder{
		FileSystem: fs,
		inodes:     make(map[filesystem.GenericInode]filesystem.InodeNumber),
		sortedMap:  make(map[*filesystem.DirectoryInode]bool),
	}
	// Create a default top-level directory which may be updated.
	b.addInode(&fs.DirectoryInode)
	// delete(fs.InodeTable, 0) // TODO(rgooch): consider this.
	return b
}

func (b *Builder) addDirectoryEntry(parent *filesystem.DirectoryInode,
	leafName string, inode filesystem.GenericInode) error {
	inodeNumber, ok := b.inodes[inode]
	if !ok {
		return fmt.Errorf("inode not yet added: %s", inode)
	}
	if parent == nil {
		parent = &b.FileSystem.DirectoryInode
	}
	if parent.EntriesByName == nil {
		parent.EntriesByName = make(map[string]*filesystem.DirectoryEntry)
	}
	if _, ok := parent.EntriesByName[leafName]; ok {
		return fmt.Errorf("cannot add duplicate entry: \"%s\" to directory: %p",
			leafName, parent)
	}
	newEntry := &filesystem.DirectoryEntry{
		Name:        leafName,
		InodeNumber: uint64(inodeNumber),
	}
	newEntry.SetInode(inode)
	if sorted, ok := b.sortedMap[parent]; !ok {
		b.sortedMap[parent] = true
	} else if sorted {
		if leafName < parent.EntryList[len(parent.EntryList)-1].Name {
			b.sortedMap[parent] = false
		}
	}
	parent.EntryList = append(parent.EntryList, newEntry)
	parent.EntriesByName[leafName] = newEntry
	return nil
}

func (b *Builder) addInode(inode filesystem.GenericInode) error {
	if _, ok := b.inodes[inode]; ok {
		return nil
	}
	switch inode := inode.(type) {
	case *filesystem.DirectoryInode:
		b.FileSystem.DirectoryCount++
	case *filesystem.RegularInode:
		b.FileSystem.NumRegularInodes++
		b.FileSystem.TotalDataBytes += inode.Size
	}
	b.inodes[inode] = b.nextInodeNumber
	b.FileSystem.InodeTable[uint64(b.nextInodeNumber)] = inode
	b.nextInodeNumber++
	return nil
}

func (b *Builder) sort() {
	for directory, sorted := range b.sortedMap {
		if !sorted {
			sort.Slice(directory.EntryList, func(left, right int) bool {
				return directory.EntryList[left].Name <
					directory.EntryList[right].Name
			})
		}
	}
	b.sortedMap = nil
}
