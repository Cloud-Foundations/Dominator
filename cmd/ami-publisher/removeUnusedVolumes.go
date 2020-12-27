package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagepublishers/amipublisher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func removeUnusedVolumesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := removeUnusedVolumes(logger); err != nil {
		return fmt.Errorf("error removing unused volumes: %s", err)
	}
	return nil
}

func removeUnusedVolumes(logger log.DebugLogger) error {
	return amipublisher.RemoveUnusedVolumes(targets, skipTargets, *instanceName,
		logger)
}
