package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func startAutoBuildsSubcommand(args []string, logger log.DebugLogger) error {
	if err := startAutoBuilds(logger); err != nil {
		return fmt.Errorf("error starting auto builds: %s", err)
	}
	return nil
}

func startAutoBuilds(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	return client.StartAutoBuilds(srpcClient, proto.StartAutoBuildsRequest{
		WaitForComplete: *waitForAutoRebuilds,
		WaitToStart:     *waitToStartAutoRebuilds,
	})
}
