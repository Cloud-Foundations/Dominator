package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	terminalclient "github.com/Cloud-Foundations/Dominator/lib/net/terminal/client"
)

func connectToVmManagerSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := connectToVmManager(args[0], logger); err != nil {
		return fmt.Errorf("error connecting to VM manager: %s", err)
	}
	return nil
}

func connectToVmManager(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return connectToVmManagerOnHypervisor(hypervisor, vmIP, logger)
	}
}

func connectToVmManagerOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	err = hyperclient.ConnectToVmManager(hypervisor, ipAddr,
		func(conn hyperclient.FlushReadWriter) error {
			return terminalclient.StartTerminal(conn)
		})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "\r")
	return nil
}
