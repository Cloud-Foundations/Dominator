package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func disableAutoBuildsSubcommand(args []string, logger log.DebugLogger) error {
	if err := disableAutoBuilds(logger); err != nil {
		return fmt.Errorf("error disabling auto builds: %s", err)
	}
	return nil
}

func disableAutoBuilds(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	_, err := client.DisableAutoBuilds(srpcClient, *disableFor)
	return err
}

func disableBuildRequestsSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := disableBuildRequests(logger); err != nil {
		return fmt.Errorf("error disabling build requests: %s", err)
	}
	return nil
}

func disableBuildRequests(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	_, err := client.DisableBuildRequests(srpcClient, *disableFor)
	return err
}
