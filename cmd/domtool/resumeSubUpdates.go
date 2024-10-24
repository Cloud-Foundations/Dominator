package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func resumeSubUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := resumeSubUpdates(client, args[0]); err != nil {
		return fmt.Errorf("error pausing updates: %s", err)
	}
	return nil
}

func resumeSubUpdates(client srpc.ClientI, hostname string) error {
	request := mdbserver.ResumeUpdatesRequest{Hostname: hostname}
	var reply mdbserver.ResumeUpdatesResponse
	err := client.RequestReply("MdbServer.ResumeUpdates", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
