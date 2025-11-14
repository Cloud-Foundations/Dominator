package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmVolumeStorageConfigurationSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := getVmVolumeStorageConfiguration(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM volume storage configuration: %s",
			err)
	}
	return nil
}

func getVmVolumeStorageConfiguration(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmVolumeStorageConfigurationOnHypervisor(hypervisor, vmIP,
			logger)
	}
}

func getVmVolumeStorageConfigurationOnHypervisor(hypervisor string,
	ipAddr net.IP, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	config, err := hyperclient.GetVmVolumeStorageConfiguration(client, ipAddr)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", config)
}
