package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func patchVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := patchVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error patching VM image: %s", err)
	}
	return nil
}

func patchVmImage(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return patchVmImageOnHypervisor(hypervisor, vmIP, logger)
	}
}

func patchVmImageOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.PatchVmImageRequest{
		ImageName:    *imageName,
		ImageTimeout: *imageTimeout,
		IpAddress:    ipAddr,
		SkipBackup:   *skipBackup,
	}
	if overlayFiles, err := loadOverlayFiles(); err != nil {
		return err
	} else {
		request.OverlayFiles = overlayFiles
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = hyperclient.PatchVmImage(client, request, logger)
	if err != nil {
		return err
	}
	if *patchLogFilename == "" {
		return nil
	}
	patchLog, _, err := hyperclient.GetVmLastPatchLog(client, ipAddr)
	if err != nil {
		return err
	}
	file, err := os.Create(*patchLogFilename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(patchLog)
	return err
}
