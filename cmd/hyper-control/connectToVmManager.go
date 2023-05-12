package main

import (
	"fmt"
	"net"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	terminalclient "github.com/Cloud-Foundations/Dominator/lib/net/terminal/client"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
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
	conn, err := client.Call("Hypervisor.ConnectToVmManager")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := proto.ConnectToVmManagerRequest{IpAddress: ipAddr}
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var response proto.ConnectToVmManagerResponse
	if err := conn.Decode(&response); err != nil {
		return err
	}
	if err := errors.New(response.Error); err != nil {
		return err
	}
	if err := terminalclient.StartTerminal(conn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "\r")
	return nil
}
