package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
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
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmOwnerGroups(client, ipAddr, ownerGroups)
}
