package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func becomePrimaryVmOwnerSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := becomePrimaryVmOwner(args[0], logger); err != nil {
		return fmt.Errorf("error becoming primary VM owner: %s", err)
	}
	return nil
}

func becomePrimaryVmOwner(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return becomePrimaryVmOwnerOnHypervisor(hypervisor, vmIP, logger)
	}
}

func becomePrimaryVmOwnerOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.BecomePrimaryVmOwnerRequest{ipAddr}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.BecomePrimaryVmOwnerResponse
	err = client.RequestReply("Hypervisor.BecomePrimaryVmOwner", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
