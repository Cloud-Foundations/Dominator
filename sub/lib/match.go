package lib

import (
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func matchTriggersInUpdate(request sub.UpdateRequest) []*triggers.Trigger {
	if request.Triggers == nil {
		return nil
	}
	for _, dir := range request.DirectoriesToMake {
		request.Triggers.Match(dir.Name)
	}
	for _, inode := range request.InodesToMake {
		request.Triggers.Match(inode.Name)
	}
	for _, hardlink := range request.HardlinksToMake {
		request.Triggers.Match(hardlink.NewLink)
	}
	for _, pathname := range request.PathsToDelete {
		request.Triggers.Match(pathname)
	}
	for _, inode := range request.InodesToChange {
		request.Triggers.Match(inode.Name)
	}
	return request.Triggers.GetMatchedTriggers()
}
