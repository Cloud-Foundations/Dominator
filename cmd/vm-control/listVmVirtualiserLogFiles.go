package main

import (
	"fmt"
	"net"
	"sort"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func listVmVirtualiserLogFilesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := listVmVirtualiserLogFiles(args[0], logger); err != nil {
		return fmt.Errorf("error listing VM virtualiser log files: %s", err)
	}
	return nil
}

func listVmVirtualiserLogFiles(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return listVmVirtualiserLogFilesOnHypervisor(hypervisor, vmIP, logger)
	}
}

func listVmVirtualiserLogFilesOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	filenames, _, err := hyperclient.ListVmVirtualiserLogFiles(client, ipAddr)
	if err != nil {
		return err
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		fmt.Println(filename)
	}
	return nil
}
