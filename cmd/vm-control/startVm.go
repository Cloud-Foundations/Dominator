package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func startVmSubcommand(args []string, logger log.DebugLogger) error {
	if err := startVm(args[0], logger); err != nil {
		return fmt.Errorf("error starting VM: %s", err)
	}
	return nil
}

func startVm(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return startVmOnHypervisor(hypervisor, vmIP, logger)
	}
}

func startVmOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	dhcpTimedOut, err := hyperclient.StartVmDhcpTimeout(client, ipAddr, nil,
		*dhcpTimeout)
	if err != nil {
		return err
	}
	if dhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	return maybeWatchVm(client, hypervisor, ipAddr, logger)
}
