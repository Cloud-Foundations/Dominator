package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	_ "github.com/Cloud-Foundations/Dominator/proto/sub"
)

func scanVmRootSubcommand(args []string, logger log.DebugLogger) error {
	if err := scanVmRoot(args[0], logger); err != nil {
		return fmt.Errorf("error scanning VM root: %s", err)
	}
	return nil
}

func scanVmRoot(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return scanVmRootOnHypervisor(hypervisor, vmIP, logger)
	}
}

func scanVmRootOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if *scanFilename == "" {
		return fmt.Errorf("no scanFilename specified")
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	fs, err := hyperclient.ScanVmRoot(client, ipAddr, nil)
	if err != nil {
		return err
	}
	file, err := fsutil.CreateRenamingWriter(*scanFilename,
		fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	if err := gob.NewEncoder(writer).Encode(fs); err != nil {
		file.Abort()
		return err
	}
	return writer.Flush()
}
