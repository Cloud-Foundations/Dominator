package analysis

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
)

func getStatisticsForFileSystems(fileSystems []*filesystem.FileSystem) (
	Statistics, error) {
	objects := make(map[hash.Hash]uint64)
	var statistics Statistics
	for _, fs := range fileSystems {
		for _, inode := range fs.InodeTable {
			if inode, ok := inode.(*filesystem.RegularInode); ok {
				statistics.NumFileInodes++
				if inode.Size < 1 {
					continue
				}
				statistics.TotalFileInodeBytes += inode.Size
				if size, ok := objects[inode.Hash]; ok {
					if inode.Size != size {
						return Statistics{},
							fmt.Errorf("Size %d != %d for hash: %x",
								inode.Size, size, inode.Hash)
					}
				} else {
					objects[inode.Hash] = inode.Size
				}
			}
		}
	}
	for _, size := range objects {
		statistics.NumObjects++
		statistics.TotalObjectBytes += size
	}
	return statistics, nil
}
