package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func forceDisruptiveUpdateSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := forceDisruptiveUpdate(getClient(), args[0]); err != nil {
		return fmt.Errorf("error forcing disruptive update: %s", err)
	}
	return nil
}

func forceDisruptiveUpdate(client *srpc.Client, subHostname string) error {
	var request dominator.ForceDisruptiveUpdateRequest
	var reply dominator.ForceDisruptiveUpdateResponse
	request.Hostname = subHostname
	return client.RequestReply("Dominator.ForceDisruptiveUpdate", request,
		&reply)
}
