package main

import (
	"errors"
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func patchVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := patchVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error patching VM image: %s", err)
	}
	return nil
}

func callPatchVmImage(client *srpc.Client, request proto.PatchVmImageRequest,
	reply *proto.PatchVmImageResponse, logger log.DebugLogger) error {
	conn, err := client.Call("Hypervisor.PatchVmImage")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	for {
		var response proto.PatchVmImageResponse
		if err := conn.Decode(&response); err != nil {
			return err
		}
		if response.Error != "" {
			return errors.New(response.Error)
		}
		if response.ProgressMessage != "" {
			logger.Debugln(0, response.ProgressMessage)
		}
		if response.Final {
			*reply = response
			return nil
		}
	}
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
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.PatchVmImageResponse
	err = callPatchVmImage(client, request, &reply, logger)
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
