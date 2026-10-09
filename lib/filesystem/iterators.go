package filesystem

import (
	"path"
)

func (directory *DirectoryInode) forEachDirectory(directoryName string,
	fn func(name string, inodeNumber uint64,
		inode *DirectoryInode) error) error {
	for _, dirent := range directory.EntryList {
		inode, ok := dirent.inode.(*DirectoryInode)
		if !ok {
			continue
		}
		name := path.Join(directoryName, dirent.Name)
		if err := fn(name, dirent.InodeNumber, inode); err != nil {
			return err
		}
		if err := inode.forEachDirectory(name, fn); err != nil {
			return err
		}
	}
	return nil
}

func (directory *DirectoryInode) forEachEntry(directoryName string,
	fn func(name string, inodeNumber uint64, inode GenericInode) error) error {
	for _, dirent := range directory.EntryList {
		name := path.Join(directoryName, dirent.Name)
		if err := fn(name, dirent.InodeNumber, dirent.inode); err != nil {
			return err
		}
		if inode, ok := dirent.inode.(*DirectoryInode); ok {
			if err := inode.forEachEntry(name, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fs *FileSystem) forEachDirectory(
	fn func(name string, inodeNumber uint64,
		inode *DirectoryInode) error) error {
	if err := fn("/", 0, &fs.DirectoryInode); err != nil {
		return err
	}
	return fs.DirectoryInode.forEachDirectory("/", fn)
}

func (fs *FileSystem) forEachFile(
	fn func(name string, inodeNumber uint64, inode GenericInode) error) error {
	if err := fn("/", 0, &fs.DirectoryInode); err != nil {
		return err
	}
	return fs.DirectoryInode.forEachEntry("/", fn)
}
