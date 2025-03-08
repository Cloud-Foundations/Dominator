package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func forceDisruptiveUpdateSubcommand(args []string,
	logger log.DebugLogger) error {
	client := getClient()
	if err := domclient.ForceDisruptiveUpdate(client, args[0]); err != nil {
		return fmt.Errorf("error forcing disruptive update: %s", err)
	}
	return nil
}
