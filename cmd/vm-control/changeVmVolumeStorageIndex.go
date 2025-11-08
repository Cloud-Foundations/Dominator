package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmVolumeStorageIndexSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmVolumeStorageIndex(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM volume storage index: %s", err)
	}
	return nil
}

func changeVmVolumeStorageIndex(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmVolumeStorageIndexOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmVolumeStorageIndexOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmVolumeStorageIndex(client, ipAddr, *storageIndex,
		*volumeIndex)
}
