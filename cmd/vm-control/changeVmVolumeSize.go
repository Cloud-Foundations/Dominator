package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmVolumeSizeSubcommand(args []string, logger log.DebugLogger) error {
	if err := changeVmVolumeSize(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM volume size: %s", err)
	}
	return nil
}

func changeVmVolumeSize(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmVolumeSizeOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmVolumeSizeOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmVolumeSize(client, ipAddr, *volumeIndex,
		uint64(volumeSize))
}
