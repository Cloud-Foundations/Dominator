package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
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
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.BecomePrimaryVmOwner(client, ipAddr)
}
