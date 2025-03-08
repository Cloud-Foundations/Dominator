package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func enableUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	if err := domclient.EnableUpdates(getClient(), args[0]); err != nil {
		return fmt.Errorf("error enabling updates: %s", err)
	}
	return nil
}
