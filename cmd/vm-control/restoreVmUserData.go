package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func restoreVmUserDataSubcommand(args []string, logger log.DebugLogger) error {
	if err := restoreVmUserData(args[0], logger); err != nil {
		return fmt.Errorf("error restoring VM user data: %s", err)
	}
	return nil
}

func restoreVmUserData(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return restoreVmUserDataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func restoreVmUserDataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.RestoreVmUserData(client, ipAddr)
}
