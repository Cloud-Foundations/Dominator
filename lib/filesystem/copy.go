package filesystem

func (inode *ComputedRegularInode) copy(newFS *FileSystem) GenericInode {
	newInode := *inode
	return &newInode
}

func (fs *FileSystem) copy() *FileSystem {
	newFS := new(FileSystem)
	newFS.InodeTable = make(InodeTable)
	newDirectoryInode := fs.DirectoryInode.copy(newFS)
	newFS.DirectoryInode = *(newDirectoryInode.(*DirectoryInode))
	newFS.NumRegularInodes = fs.NumRegularInodes
	newFS.TotalDataBytes = fs.TotalDataBytes
	return newFS
}

func (inode *DirectoryInode) copy(newFS *FileSystem) GenericInode {
	newInode := &DirectoryInode{
		Mode: inode.Mode,
		Uid:  inode.Uid,
		Gid:  inode.Gid,
	}
	for _, entry := range inode.EntryList {
		newEntry := &DirectoryEntry{
			Name:        entry.Name,
			InodeNumber: entry.InodeNumber,
		}
		if entryInode, ok := newFS.InodeTable[newEntry.InodeNumber]; !ok {
			newEntry.inode = entry.inode.copy(newFS)
			newFS.InodeTable[newEntry.InodeNumber] = newEntry.inode
		} else {
			newEntry.inode = entryInode
		}
		newInode.EntryList = append(newInode.EntryList, newEntry)
	}
	newFS.DirectoryCount++
	return newInode
}

func (inode *RegularInode) copy(newFS *FileSystem) GenericInode {
	newInode := *inode
	return &newInode
}

func (inode *SpecialInode) copy(newFS *FileSystem) GenericInode {
	newInode := *inode
	return &newInode
}

func (inode *SymlinkInode) copy(newFS *FileSystem) GenericInode {
	newInode := *inode
	return &newInode
}
