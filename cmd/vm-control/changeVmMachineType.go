package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmMachineTypeSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmMachineType(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM machine type: %s", err)
	}
	return nil
}

func changeVmMachineType(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmMachineTypeOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmMachineTypeOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmMachineType(client, ipAddr, machineType)
}
