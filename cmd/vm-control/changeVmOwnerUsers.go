package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmOwnerUsersSubcommand(args []string, logger log.DebugLogger) error {
	if err := changeVmOwnerUsers(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM owner users: %s", err)
	}
	return nil
}

func changeVmOwnerUsers(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmOwnerUsersOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmOwnerUsersOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmOwnerUsers(client, ipAddr, ownerUsers)
}
