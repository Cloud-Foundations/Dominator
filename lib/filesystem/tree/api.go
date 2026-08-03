package tree

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/fstree"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func Get(getter fstree.Getter, treeUrl string, logger log.DebugLogger) (
	*filesystem.FileSystem, []hash.Hash, []uint64, error) {
	return get(getter, treeUrl, logger)
}
