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
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	conn, err := client.Call("Hypervisor.ConnectToVmSerialPort")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := proto.ConnectToVmSerialPortRequest{
		IpAddress:  ipAddr,
		PortNumber: *serialPort,
	}
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var response proto.ConnectToVmSerialPortResponse
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
