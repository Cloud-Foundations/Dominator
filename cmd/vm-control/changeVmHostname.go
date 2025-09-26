package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmHostnameSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmHostname(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM hostname: %s", err)
	}
	return nil
}

func changeVmHostname(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmHostnameOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmHostnameOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmHostname(client, ipAddr, *vmHostname)
}
