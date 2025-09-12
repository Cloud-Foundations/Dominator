package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func discardVmOldUserDataSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := discardVmOldUserData(args[0], logger); err != nil {
		return fmt.Errorf("error discarding VM old user data: %s", err)
	}
	return nil
}

func discardVmOldUserData(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return discardVmOldUserDataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func discardVmOldUserDataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.DiscardVmOldUserData(client, ipAddr)
}
