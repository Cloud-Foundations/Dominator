package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	terminalclient "github.com/Cloud-Foundations/Dominator/lib/net/terminal/client"
)

func connectToVmSerialPortSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := connectToVmSerialPort(args[0], logger); err != nil {
		return fmt.Errorf("error connecting to VM serial port: %s", err)
	}
	return nil
}

func connectToVmSerialPort(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return connectToVmSerialPortOnHypervisor(hypervisor, vmIP, logger)
	}
}

func connectToVmSerialPortOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	err := hyperclient.ConnectToVmSerialPort(hypervisor, ipAddr, *serialPort,
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
