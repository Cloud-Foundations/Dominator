package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func truncateVmVirtualiserLogFileSubcommand(args []string,
	logger log.DebugLogger) error {
	if e := truncateVmVirtualiserLogFile(args[0], args[1], logger); e != nil {
		return fmt.Errorf("error truncating VM virtualiser log file: %s: %s",
			args[1], e)
	}
	return nil
}

func truncateVmVirtualiserLogFile(vmHostname string, filename string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return truncateVmVirtualiserLogFileOnHypervisor(hypervisor, vmIP,
			filename, logger)
	}
}

func truncateVmVirtualiserLogFileOnHypervisor(hypervisor string, ipAddr net.IP,
	filename string, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.TruncateVmVirtualiserLogFile(client,
		ipAddr, filename)
}
