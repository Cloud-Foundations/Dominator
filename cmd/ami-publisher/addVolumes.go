package main

import (
	"fmt"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/imagepublishers/amipublisher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func addVolumesSubcommand(args []string, logger log.DebugLogger) error {
	if err := addVolumes(args[0], logger); err != nil {
		return fmt.Errorf("error adding volumes: %s", err)
	}
	return nil
}

func addVolumes(sizeStr string, logger log.DebugLogger) error {
	sizeInGiB, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return err
	}
	return amipublisher.AddVolumes(targets, skipTargets, tags,
		*instanceName, uint64(sizeInGiB)<<30, logger)
}
