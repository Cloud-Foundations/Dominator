package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmVolumeInterfacesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmVolumeInterfaces(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM volume interfaces: %s", err)
	}
	return nil
}

func changeVmVolumeInterfaces(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmVolumeInterfacesOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmVolumeInterfacesOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmVolumeInterfaces(client, ipAddr,
		volumeInterfaces)
}
