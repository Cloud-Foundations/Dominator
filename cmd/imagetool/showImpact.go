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

// The most disruptive levels come first. Reboot and high impact are disruptive.
type impactLevel uint

const (
	levelReboot impactLevel = iota
	levelHighImpact
	levelRestart
	levelReload
)

type impactSummary struct {
	DoReboot               bool     `json:",omitempty"`
	HighlyImpactedServices []string `json:",omitempty"`
}

type impactDetail struct {
	impactSummary
	Triggers []*triggers.Trigger
}

func showImpactSubcommand(args []string, logger log.DebugLogger) error {
	trgs, err := showImpact(args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error showing impact: %s", err)
	}
	summary := makeSummary(trgs)
	if *showTriggers {
		return json.WriteWithIndent(os.Stdout, "    ",
			impactDetail{summary, trgs})
	}
	if summary.DoReboot { // The reboot restarts every service.
		summary.HighlyImpactedServices = nil
	}
	return json.WriteWithIndent(os.Stdout, "    ", summary)
}

func showImpact(lName, rName string, logger log.DebugLogger) (
	[]*triggers.Trigger, error) {
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
	return sortByImpact(sublib.MatchTriggersInUpdate(request)), nil
}

func sortByImpact(matched []*triggers.Trigger) []*triggers.Trigger {
	restarting := make(map[string]struct{})
	for _, trg := range matched {
		if !trg.DoReload {
			restarting[trg.Service] = struct{}{}
		}
	}
	impacts := make([]*triggers.Trigger, 0, len(matched))
	for _, trg := range matched {
		if _, ok := restarting[trg.Service]; ok && trg.DoReload {
			continue // This service will be restarted: skip reload.
		}
		impacts = append(impacts, trg)
	}
	sort.Slice(impacts, func(left, right int) bool {
		lLevel := getImpactLevel(impacts[left])
		rLevel := getImpactLevel(impacts[right])
		if lLevel != rLevel {
			return lLevel < rLevel
		}
		return impacts[left].Service < impacts[right].Service
	})
	return impacts
}

func getImpactLevel(trg *triggers.Trigger) impactLevel {
	highImpact, reboot := sublib.CheckImpact([]*triggers.Trigger{trg})
	switch {
	case reboot:
		return levelReboot
	case highImpact:
		return levelHighImpact
	case trg.DoReload:
		return levelReload
	default:
		return levelRestart
	}
}

func makeSummary(trgs []*triggers.Trigger) impactSummary {
	var summary impactSummary
	for _, trg := range trgs {
		switch getImpactLevel(trg) {
		case levelReboot:
			summary.DoReboot = true
		case levelHighImpact:
			services := summary.HighlyImpactedServices
			if len(services) < 1 || services[len(services)-1] != trg.Service {
				summary.HighlyImpactedServices = append(services, trg.Service)
			}
		}
	}
	return summary
}
