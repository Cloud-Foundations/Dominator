package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
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
	hostnames, err := getSubsFromFile()
	if err != nil {
		return err
	}
	request := dominator.ListSubsRequest{
		Hostnames:        hostnames,
		LocationsToMatch: locationsToMatch,
		StatusesToMatch:  statusesToMatch,
		TagsToMatch:      tagsToMatch,
	}
	hostnames, err = domclient.ListSubs(client, request)
	if err != nil {
		return err
	}
	for _, hostname := range hostnames {
		fmt.Println(hostname)
	}
	return nil
}
