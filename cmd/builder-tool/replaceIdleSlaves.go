package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func replaceIdleSlavesSubcommand(args []string, logger log.DebugLogger) error {
	if err := replaceIdleSlaves(logger); err != nil {
		return fmt.Errorf("error replacing idle slaves: %s", err)
	}
	return nil
}

func replaceIdleSlaves(logger log.Logger) error {
	srpcClient := getImaginatorClient()
	return client.ReplaceIdleSlaves(srpcClient, true)
}
