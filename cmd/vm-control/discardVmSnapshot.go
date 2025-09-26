package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func discardVmSnapshotSubcommand(args []string, logger log.DebugLogger) error {
	if err := discardVmSnapshot(args[0], logger); err != nil {
		return fmt.Errorf("error discarding VM snapshot: %s", err)
	}
	return nil
}

func discardVmSnapshot(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return discardVmSnapshotOnHypervisor(hypervisor, vmIP, logger)
	}
}

func discardVmSnapshotOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.DiscardVmSnapshot(client, ipAddr, *snapshotName)
}
