package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmVolumeSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmVolume(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM volume: %s", err)
	}
	return nil
}

func getVmVolume(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmVolumeOnHypervisor(hypervisor, vmIP, logger)
	}
}

func getVmVolumeOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	vmInfo, err := hyperclient.GetVmInfo(client, ipAddr)
	if err != nil {
		return err
	}
	if *volumeIndex >= uint(len(vmInfo.Volumes)) {
		return fmt.Errorf("volumeIndex too large")
	}
	return copyVolumeToVmSaver(&directorySaver{filename: *volumeFilename},
		client, ipAddr, *volumeIndex, vmInfo.Volumes[*volumeIndex].Size, logger)
}
