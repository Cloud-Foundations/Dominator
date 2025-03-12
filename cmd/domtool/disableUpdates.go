package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func disableUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	if err := domclient.DisableUpdates(getClient(), args[0]); err != nil {
		return fmt.Errorf("error disabling updates: %s", err)
	}
	return nil
}
