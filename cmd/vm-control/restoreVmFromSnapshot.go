package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func restoreVmFromSnapshotSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := restoreVmFromSnapshot(args[0], logger); err != nil {
		return fmt.Errorf("error restoring VM from snapshot: %s", err)
	}
	return nil
}

func restoreVmFromSnapshot(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return restoreVmFromSnapshotOnHypervisor(hypervisor, vmIP, logger)
	}
}

func restoreVmFromSnapshotOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.RestoreVmFromSnapshotRequest{
		IpAddress:         ipAddr,
		ForceIfNotStopped: *forceIfNotStopped,
		Name:              *snapshotName,
		Retain:            *retain,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.RestoreVmFromSnapshot(client, request)
}
