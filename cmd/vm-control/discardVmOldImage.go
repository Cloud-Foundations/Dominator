package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func discardVmOldImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := discardVmOldImage(args[0], logger); err != nil {
		return fmt.Errorf("error discarding VM old image: %s", err)
	}
	return nil
}

func discardVmOldImage(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return discardVmOldImageOnHypervisor(hypervisor, vmIP, logger)
	}
}

func discardVmOldImageOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.DiscardVmOldImage(client, ipAddr)
}
