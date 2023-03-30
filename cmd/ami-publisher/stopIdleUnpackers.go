package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagepublishers/amipublisher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func stopIdleUnpackersSubcommand(args []string, logger log.DebugLogger) error {
	err := amipublisher.StopIdleUnpackers(targets, skipTargets, *instanceName,
		*maxIdleTime, logger)
	if err != nil {
		return fmt.Errorf("error stopping idle unpackers: %s", err)
	}
	return nil
}
