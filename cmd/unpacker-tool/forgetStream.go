package main

import (
	"fmt"

	uclient "github.com/Cloud-Foundations/Dominator/imageunpacker/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func forgetStreamSubcommand(args []string, logger log.DebugLogger) error {
	if err := uclient.ForgetStream(getClient(), args[0]); err != nil {
		return fmt.Errorf("error forgetting stream: %s", err)
	}
	return nil
}
