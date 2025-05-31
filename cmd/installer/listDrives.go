package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func listDrivesSubcommand(args []string, logger log.DebugLogger) error {
	if err := listDrivesCmd(logger); err != nil {
		return fmt.Errorf("error listing drives: %s", err)
	}
	return nil
}

func listDrivesCmd(logger log.DebugLogger) error {
	drives, err := listDrives(logger)
	if err != nil {
		return err
	}
	for _, drive := range drives {
		fmt.Println(drive.name)
	}
	return nil
}
