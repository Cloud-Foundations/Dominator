package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageunpacker/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func prepareForCopySubcommand(args []string, logger log.DebugLogger) error {
	if err := client.PrepareForCopy(getClient(), args[0]); err != nil {
		return fmt.Errorf("error preparing for copy: %s", err)
	}
	return nil
}
