package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func enableBuildRequestsSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := enableBuildRequests(logger); err != nil {
		return fmt.Errorf("error enabling build requests: %s", err)
	}
	return nil
}

func enableBuildRequests(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	_, err := client.DisableBuildRequests(srpcClient, 0)
	return err
}
