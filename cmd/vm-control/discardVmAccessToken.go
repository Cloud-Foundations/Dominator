package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func discardVmAccessTokenSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := discardVmAccessToken(args[0], logger); err != nil {
		return fmt.Errorf("error discarding VM access token: %s", err)
	}
	return nil
}

func discardVmAccessToken(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return discardVmAccessTokenOnHypervisor(hypervisor, vmIP, logger)
	}
}

func discardVmAccessTokenOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.DiscardVmAccessToken(client, ipAddr, nil)
}
