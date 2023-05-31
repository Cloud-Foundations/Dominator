package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func holdVmLockSubcommand(args []string, logger log.DebugLogger) error {
	if err := holdVmLock(args[0], logger); err != nil {
		return fmt.Errorf("error holding VM lock: %s", err)
	}
	return nil
}

func holdVmLock(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return holdVmLockOnHypervisor(hypervisor, vmIP, logger)
	}
}

func holdVmLockOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.HoldVmLock(client, ipAddr, *lockTimeout, *writeLock)
}
