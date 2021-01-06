package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func restoreVmImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := restoreVmImage(args[0], logger); err != nil {
		return fmt.Errorf("error restoring VM image: %s", err)
	}
	return nil
}

func restoreVmImage(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return restoreVmImageOnHypervisor(hypervisor, vmIP, logger)
	}
}

func restoreVmImageOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.RestoreVmImageRequest{ipAddr}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.RestoreVmImageResponse
	err = client.RequestReply("Hypervisor.RestoreVmImage", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
