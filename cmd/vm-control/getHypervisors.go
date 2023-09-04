package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func getHypervisorsSubcommand(args []string, logger log.DebugLogger) error {
	if err := getHypervisors(logger); err != nil {
		return fmt.Errorf("error geting Hypervisors: %s", err)
	}
	return nil
}

func getHypervisors(logger log.DebugLogger) error {
	fleetManager := fmt.Sprintf("%s:%d",
		*fleetManagerHostname, *fleetManagerPortNum)
	client, err := dialFleetManager(fleetManager)
	if err != nil {
		return err
	}
	defer client.Close()
	request := proto.GetHypervisorsInLocationRequest{
		HypervisorTagsToMatch: hypervisorTagsToMatch,
		IncludeUnhealthy:      *includeUnhealthy,
		IncludeVMs:            true,
		Location:              *location,
		SubnetId:              *subnetId,
	}
	var reply proto.GetHypervisorsInLocationResponse
	err = client.RequestReply("FleetManager.GetHypervisorsInLocation",
		request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", reply.Hypervisors)
}
