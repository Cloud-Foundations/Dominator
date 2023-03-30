package main

import (
	"fmt"

	uclient "github.com/Cloud-Foundations/Dominator/imageunpacker/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func claimDeviceSubcommand(args []string, logger log.DebugLogger) error {
	if err := uclient.ClaimDevice(getClient(), args[0], args[1]); err != nil {
		return fmt.Errorf("error claiming device: %s", err)
	}
	return nil
}
