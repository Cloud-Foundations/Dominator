package lib

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

type DisruptionCancelor func()
type DisruptionRequestor func() sub.DisruptionState

type TriggersRunner func(triggers []*triggers.Trigger, action string,
	logger log.Logger) bool

type UpdateOptions struct {
	DisruptionCancel  DisruptionCancelor
	DisruptionRequest DisruptionRequestor
	Logger            log.Logger
	ObjectsDir        string
	OldTriggers       *triggers.Triggers
	RootDirectoryName string
	RunTriggers       TriggersRunner
	SkipFilter        *filter.Filter
}

type uType struct {
	UpdateOptions
	disableTriggers    bool
	lastError          error
	hadTriggerFailures bool
	fsChangeDuration   time.Duration
}

// CheckImpact will return whether any trigger has high impact or will reboot.
func CheckImpact(triggerList []*triggers.Trigger) (highImpact, reboot bool) {
	return checkImpact(triggerList)
}

// MatchTriggersInUpdate will return a list of triggers in an update request
// that match the list of changes. Since there is no file-system to compare to,
// potential mtime-only changes will also match.
func MatchTriggersInUpdate(request sub.UpdateRequest) []*triggers.Trigger {
	return matchTriggersInUpdate(request)
}

// Update is deprecated. Use UpdateWithOptions instead.
func Update(request sub.UpdateRequest, rootDirectoryName string,
	objectsDir string, oldTriggers *triggers.Triggers,
	skipFilter *filter.Filter, triggersRunner TriggersRunner,
	logger log.Logger) (
	bool, time.Duration, error) {
	options := UpdateOptions{
		Logger:            logger,
		ObjectsDir:        objectsDir,
		OldTriggers:       oldTriggers,
		RootDirectoryName: rootDirectoryName,
		RunTriggers:       triggersRunner,
		SkipFilter:        skipFilter,
	}
	return UpdateWithOptions(request, options)
}

// UpdateWithOptions will process an update request, modifying the local
// file-system and running triggers.
func UpdateWithOptions(request sub.UpdateRequest, options UpdateOptions) (
	bool, time.Duration, error) {
	updateObj := &uType{UpdateOptions: options}
	err := updateObj.update(request)
	return updateObj.hadTriggerFailures, updateObj.fsChangeDuration, err
}
