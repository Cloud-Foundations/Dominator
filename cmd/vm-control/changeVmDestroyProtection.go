package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmDestroyProtectionSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmDestroyProtection(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM destroy protection: %s", err)
	}
	return nil
}

func changeVmDestroyProtection(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmDestroyProtectionOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmDestroyProtectionOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmDestroyProtection(client, ipAddr,
		*destroyProtection)
}
