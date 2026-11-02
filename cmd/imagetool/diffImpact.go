package main

import (
	"errors"
	"fmt"
	"os"
	"sort"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

type impactReport struct {
	Disruptive []string
	Other      []string
}

type impact struct {
	trigger    *triggers.Trigger
	disruptive bool
}

func diffImpactSubcommand(args []string, logger log.DebugLogger) error {
	impacts, err := diffImpact(args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error diffing impact: %s", err)
	}
	return printImpacts(impacts)
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

func printImpacts(impacts []*impact) error {
	return json.WriteWithIndent(os.Stdout, "    ", makeReport(impacts))
}

func makeReport(impacts []*impact) impactReport {
	report := impactReport{Disruptive: []string{}, Other: []string{}}
	for _, imp := range impacts {
		action := getAction(imp.trigger)
		if imp.disruptive {
			report.Disruptive = append(report.Disruptive, action)
		} else {
			report.Other = append(report.Other, action)
		}
	}
	return report
}
