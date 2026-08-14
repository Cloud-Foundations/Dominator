package analysis

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
)

type Statistics struct {
	NumFileInodes       uint64
	NumObjects          uint64
	TotalFileInodeBytes uint64
	TotalObjectBytes    uint64
}

// GetStatisticsForFileSystems will compute aggregated statistics for multiple
// FileSystems.
func GetStatisticsForFileSystems(fileSystems []*filesystem.FileSystem) (
	Statistics, error) {
	return getStatisticsForFileSystems(fileSystems)
}
