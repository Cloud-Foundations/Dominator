package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/sub/client"
)

func boostScanLimitSubcommand(args []string, logger log.DebugLogger) error {
	srpcClient := getSubClient(logger)
	defer srpcClient.Close()
	if err := boostScanLimit(srpcClient); err != nil {
		return fmt.Errorf("error boosting scan limit: %s", err)
	}
	return nil
}

func boostScanLimit(srpcClient *srpc.Client) error {
	return client.BoostScanLimit(srpcClient)
}
