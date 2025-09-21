package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmUserDataSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmUserData(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM user data: %s", err)
	}
	return nil
}

func getVmUserData(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmUserDataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func getVmUserDataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if *userDataFile == "" {
		return errors.New("no user data file specified")
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	conn, length, err := hyperclient.GetVmUserData(client, ipAddr, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	file, err := os.OpenFile(*userDataFile, os.O_WRONLY|os.O_CREATE,
		fsutil.PrivateFilePerms)
	if err != nil {
		io.CopyN(ioutil.Discard, conn, int64(length))
		return err
	}
	defer file.Close()
	logger.Debugln(0, "downloading user data")
	if _, err := io.CopyN(file, conn, int64(length)); err != nil {
		return err
	}
	return nil
}
