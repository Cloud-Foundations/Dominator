package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func setDefaultImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := domclient.SetDefaultImage(getClient(), args[0]); err != nil {
		return fmt.Errorf("error setting default image: %s", err)
	}
	return nil
}
