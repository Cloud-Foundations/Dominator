package filesystem

import (
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

func (fs *FileSystem) getComputedFiles() []ComputedFile {
	var computedFiles []ComputedFile
	fs.DirectoryInode.getComputedFiles(&computedFiles, "/")
	return computedFiles
}

func (fs *FileSystem) getObjects() map[hash.Hash]uint64 {
	objects := make(map[hash.Hash]uint64)
	for _, inode := range fs.InodeTable {
		if inode, ok := inode.(*RegularInode); ok {
			if inode.Size > 0 {
				objects[inode.Hash] = inode.Size
			}
		}
	}
	return objects
}

func (di *DirectoryInode) getComputedFiles(computedFiles *[]ComputedFile,
	name string) {
	for _, dirent := range di.EntryList {
		if inode, ok := dirent.Inode().(*ComputedRegularInode); ok {
			*computedFiles = append(*computedFiles, ComputedFile{
				Filename: path.Join(name, dirent.Name),
				Source:   inode.Source,
			})
		} else if inode, ok := dirent.Inode().(*DirectoryInode); ok {
			inode.getComputedFiles(computedFiles, path.Join(name, dirent.Name))
		}
	}
}
