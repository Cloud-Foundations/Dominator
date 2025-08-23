package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmCreateRequestSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmCreateRequest(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM create request: %s", err)
	}
	return nil
}

func getVmCreateRequest(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmCreateRequestOnHypervisor(hypervisor, vmIP, logger)
	}
}

func getVmCreateRequestOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	createRequest, err := hyperclient.GetVmCreateRequest(client, ipAddr)
	if err != nil {
		return err
	} else {
		return json.WriteWithIndent(os.Stdout, "    ", createRequest)
	}
}
