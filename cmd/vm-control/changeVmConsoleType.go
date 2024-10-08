package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmConsoleTypeSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmConsoleType(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM console type: %s", err)
	}
	return nil
}

func changeVmConsoleType(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmConsoleTypeOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmConsoleTypeOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmConsoleType(client, ipAddr, consoleType)
}
