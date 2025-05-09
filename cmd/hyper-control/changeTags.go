package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func changeTagsSubcommand(args []string, logger log.DebugLogger) error {
	if err := changeTags(logger); err != nil {
		return fmt.Errorf("error changing Hypervisor tags: %s", err)
	}
	return nil
}

func changeTags(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("no hypervisorHostname specified")
	}
	client, err := dialFleetManager()
	if err != nil {
		return err
	}
	defer client.Close()
	if len(hypervisorTags) < 1 {
		return setMachineTags(client, *hypervisorHostname, nil)
	}
	oldTags, err := getMachineTags(client, *hypervisorHostname)
	if err != nil {
		return err
	}
	if len(oldTags) < 1 {
		return setMachineTags(client, *hypervisorHostname, hypervisorTags)
	}
	oldTags.Merge(hypervisorTags)
	for key, value := range oldTags {
		if value == "" {
			delete(oldTags, key)
		}
	}
	return setMachineTags(client, *hypervisorHostname, oldTags)
}

func getMachineTags(client srpc.ClientI, hostname string) (tags.Tags, error) {
	request := proto.GetMachineInfoRequest{
		Hostname: hostname,
	}
	var reply proto.GetMachineInfoResponse
	err := client.RequestReply("FleetManager.GetMachineInfo", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.Machine.Tags, nil
}

func setMachineTags(client srpc.ClientI, hostname string, tgs tags.Tags) error {
	request := proto.ChangeMachineTagsRequest{
		Hostname: hostname,
		Tags:     tgs,
	}
	var reply proto.ChangeMachineTagsResponse
	err := client.RequestReply("FleetManager.ChangeMachineTags", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
