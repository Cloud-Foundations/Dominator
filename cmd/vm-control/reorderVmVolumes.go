package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func reorderVmVolumesSubcommand(args []string, logger log.DebugLogger) error {
	if err := reorderVmVolumes(args[0], logger); err != nil {
		return fmt.Errorf("error reordering VM volumes: %s", err)
	}
	return nil
}

func reorderVmVolumes(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return reorderVmVolumesOnHypervisor(hypervisor, vmIP, logger)
	}
}

func reorderVmVolumesOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if len(volumeIndices) < 1 {
		return errors.New("no volumeIndices specified")
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ReorderVmVolumes(client, ipAddr, nil, volumeIndices)
}
