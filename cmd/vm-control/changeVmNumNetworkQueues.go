package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmNumNetworkQueuesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmNumNetworkQueues(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM num network queues: %s", err)
	}
	return nil
}

func changeVmNumNetworkQueues(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmNumNetworkQueuesOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmNumNetworkQueuesOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ChangeVmNumNetworkQueues(client, ipAddr,
		numNetworkQueues)
}
