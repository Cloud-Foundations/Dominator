package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageunpacker/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func prepareForUnpackSubcommand(args []string, logger log.DebugLogger) error {
	if err := prepareForUnpack(getClient(), args[0]); err != nil {
		return fmt.Errorf("error preparing for unpack: %s", err)
	}
	return nil
}

func prepareForUnpack(srpcClient *srpc.Client, streamName string) error {
	return client.PrepareForUnpack(srpcClient, streamName, false, false)
}
