package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func clearSafetyShutoffSubcommand(args []string, logger log.DebugLogger) error {
	if err := domclient.ClearSafetyShutoff(getClient(), args[0]); err != nil {
		return fmt.Errorf("error clearing safety shutoff: %s", err)
	}
	return nil
}
