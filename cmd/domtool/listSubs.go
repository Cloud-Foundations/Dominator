package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func listSubsSubcommand(args []string, logger log.DebugLogger) error {
	if err := listSubs(getClient()); err != nil {
		return fmt.Errorf("error listing subs: %s", err)
	}
	return nil
}

func listSubs(client *srpc.Client) error {
	request := dominator.ListSubsRequest{
		StatusToMatch: *statusToMatch,
	}
	var reply dominator.ListSubsResponse
	if err := client.RequestReply("Dominator.ListSubs", request,
		&reply); err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	for _, hostname := range reply.Hostnames {
		fmt.Println(hostname)
	}
	return nil
}
