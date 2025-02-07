package lib

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func (sub *Sub) buildMissingLists(requiredImage, plannedImage *image.Image,
	pushComputedFiles, ignoreMissingComputedFiles bool, logger log.Logger) (
	map[hash.Hash]uint64, map[hash.Hash]struct{}) {
	objectsToFetch := make(map[hash.Hash]uint64)
	objectsToPush := make(map[hash.Hash]struct{})
	ok := sub.updateMissingLists(requiredImage, pushComputedFiles,
		ignoreMissingComputedFiles, objectsToFetch, objectsToPush, logger)
	if !ok {
		return nil, nil
	}
	ok = sub.updateMissingLists(plannedImage, false, false,
		objectsToFetch, objectsToPush, logger)
	if !ok {
		return nil, nil
	}
	for _, hashVal := range sub.ObjectCache {
		delete(objectsToFetch, hashVal)
		delete(objectsToPush, hashVal)
	}
	for _, inode := range sub.FileSystem.InodeTable {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				delete(objectsToFetch, inode.Hash)
				delete(objectsToPush, inode.Hash)
			}
		}
	}
	return objectsToFetch, objectsToPush
}

// Returns false if there was a problem updating the missing lists.
func (sub *Sub) updateMissingLists(img *image.Image,
	pushComputedFiles, ignoreMissingComputedFiles bool,
	objectsToFetch map[hash.Hash]uint64, objectsToPush map[hash.Hash]struct{},
	logger log.Logger) bool {
	if img == nil {
		return true
	}
	inodeToFilenamesTable := img.FileSystem.InodeToFilenamesTable()
	for inum, inode := range img.FileSystem.InodeTable {
		if rInode, ok := inode.(*filesystem.RegularInode); ok {
			if rInode.Size > 0 {
				objectsToFetch[rInode.Hash] = rInode.Size
			}
		} else if pushComputedFiles {
			if _, ok := inode.(*filesystem.ComputedRegularInode); ok {
				pathname := inodeToFilenamesTable[inum][0]
				if inode, ok := sub.ComputedInodes[pathname]; !ok {
					if ignoreMissingComputedFiles {
						continue
					}
					logger.Printf(
						"buildMissingLists(%s): missing computed file: %s\n",
						sub, pathname)
					return false
				} else {
					objectsToPush[inode.Hash] = struct{}{}
				}
			}
		}
	}
	return true
}
