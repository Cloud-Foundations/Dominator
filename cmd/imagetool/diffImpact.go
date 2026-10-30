package main

import (
	"errors"
	"fmt"
	"sort"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

type impact struct {
	trigger    *triggers.Trigger
	disruptive bool
}

func diffImpactSubcommand(args []string, logger log.DebugLogger) error {
	impacts, err := diffImpact(args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error diffing impact: %s", err)
	}
	printImpacts(impacts)
	return nil
}

func diffImpact(lName, rName string, logger log.DebugLogger) ([]*impact,
	error) {
	lfs, err := getTypedFileSystem(lName)
	if err != nil {
		return nil, fmt.Errorf("error getting left image: %s", err)
	}
	img, err := getTypedImage(rName)
	if err != nil {
		return nil, fmt.Errorf("error getting right image: %s", err)
	}
	if *ignoreFilters {
		img.Filter = &filter.Filter{}
	}
	// Treat the left image as the sub, so the filter and triggers of the right
	// image apply, as in a push.
	subObj := domlib.Sub{FileSystem: lfs}
	for _, fs := range []*filesystem.FileSystem{lfs, img.FileSystem} {
		if err := fs.RebuildInodePointers(); err != nil {
			return nil, err
		}
		fs.BuildEntryMap()
		// Objects of both images are cached, as after a fetch.
		for hashVal := range fs.GetObjects() {
			subObj.ObjectCache = append(subObj.ObjectCache, hashVal)
		}
	}
	var request subproto.UpdateRequest
	if domlib.BuildUpdateRequest(subObj, img, &request, false, true, logger) {
		return nil, errors.New("failed building update: missing computed files")
	}
	return makeImpacts(sublib.MatchTriggersInUpdate(request)), nil
}

func makeImpacts(matched []*triggers.Trigger) []*impact {
	restarting := make(map[string]struct{})
	for _, trg := range matched {
		if !trg.DoReload {
			restarting[trg.Service] = struct{}{}
		}
	}
	impacts := make([]*impact, 0, len(matched))
	for _, trg := range matched {
		if _, ok := restarting[trg.Service]; ok && trg.DoReload {
			continue // This service will be restarted: skip reload.
		}
		highImpact, reboot := sublib.CheckImpact([]*triggers.Trigger{trg})
		impacts = append(impacts, &impact{
			trigger:    trg,
			disruptive: highImpact || reboot,
		})
	}
	sort.Slice(impacts, func(left, right int) bool {
		return getAction(impacts[left].trigger) <
			getAction(impacts[right].trigger)
	})
	return impacts
}

func getAction(trg *triggers.Trigger) string {
	if trg.DoReboot {
		return "reboot"
	}
	if trg.DoReload {
		return "reload " + trg.Service
	}
	return "restart " + trg.Service
}

func printImpacts(impacts []*impact) {
	var disruptive, other []*impact
	for _, imp := range impacts {
		if imp.disruptive {
			disruptive = append(disruptive, imp)
		} else {
			other = append(other, imp)
		}
	}
	printGroup("Disruptive", disruptive)
	printGroup("Other", other)
}

func printGroup(title string, impacts []*impact) {
	fmt.Printf("%s: %d\n", title, len(impacts))
	for _, imp := range impacts {
		fmt.Printf("  %s\n", getAction(imp.trigger))
	}
}
