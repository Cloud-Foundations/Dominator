package lib

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

type DisruptionRequestor func() sub.DisruptionState

type TriggersRunner func(triggers []*triggers.Trigger, action string,
	logger log.Logger) bool

type UpdateOptions struct {
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

func UpdateWithOptions(request sub.UpdateRequest, options UpdateOptions) (
	bool, time.Duration, error) {
	updateObj := &uType{UpdateOptions: options}
	err := updateObj.update(request)
	return updateObj.hadTriggerFailures, updateObj.fsChangeDuration, err
}
