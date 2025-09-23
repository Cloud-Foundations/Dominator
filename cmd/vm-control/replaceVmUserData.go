package main

import (
	"bufio"
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func replaceVmUserDataSubcommand(args []string, logger log.DebugLogger) error {
	if err := replaceVmUserData(args[0], logger); err != nil {
		return fmt.Errorf("error replacing VM user data: %s", err)
	}
	return nil
}

func replaceVmUserData(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return replaceVmUserDataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func replaceVmUserDataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if *userDataFile == "" {
		return errors.New("no user data file specified")
	}
	file, size, err := getReader(*userDataFile)
	if err != nil {
		return err
	}
	defer file.Close()
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.ReplaceVmUserData(client, ipAddr, bufio.NewReader(file),
		uint64(size), logger)
}
