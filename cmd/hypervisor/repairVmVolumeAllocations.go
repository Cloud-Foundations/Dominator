package main

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func repairVmVolumeAllocationsSubcommand(args []string,
	logger log.DebugLogger) error {
	return checkVms(true)
}
