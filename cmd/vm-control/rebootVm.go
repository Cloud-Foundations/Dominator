package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func rebootVmSubcommand(args []string, logger log.DebugLogger) error {
	if err := rebootVm(args[0], logger); err != nil {
		return fmt.Errorf("error rebooting VM: %s", err)
	}
	return nil
}

func rebootVm(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return rebootVmOnHypervisor(hypervisor, vmIP, logger)
	}
}

func rebootVmOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	dhcpTimedOut, err := hyperclient.RebootVm(client, ipAddr, *dhcpTimeout)
	if err != nil {
		return err
	}
	if dhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	return maybeWatchVm(client, hypervisor, ipAddr, logger)
}
