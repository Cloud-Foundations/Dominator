package main

import (
	"fmt"
	"io"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmVirtualiserLogFileSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := getVmVirtualiserLogFile(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error getting VM virtualiser log file: %s: %s",
			args[1], err)
	}
	return nil
}

func getVmVirtualiserLogFile(vmHostname string, filename string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmVirtualiserLogFileOnHypervisor(hypervisor, vmIP, filename,
			logger)
	}
}

func getVmVirtualiserLogFileOnHypervisor(hypervisor string, ipAddr net.IP,
	filename string, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	readerCloser, length, err := hyperclient.GetVmVirtualiserLogFile(client,
		ipAddr, filename)
	if err != nil {
		return err
	}
	defer readerCloser.Close()
	if _, err := io.CopyN(os.Stdout, readerCloser, int64(length)); err != nil {
		return err
	}
	return nil
}
