package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func changeVmOwnerGroupsSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmOwnerGroups(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM owner groups: %s", err)
	}
	return nil
}

func changeVmOwnerGroups(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmOwnerGroupsOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmOwnerGroupsOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.ChangeVmOwnerGroupsRequest{ipAddr, ownerGroups}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.ChangeVmOwnerGroupsResponse
	err = client.RequestReply("Hypervisor.ChangeVmOwnerGroups", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
