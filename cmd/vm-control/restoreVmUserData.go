package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func restoreVmUserDataSubcommand(args []string, logger log.DebugLogger) error {
	if err := restoreVmUserData(args[0], logger); err != nil {
		return fmt.Errorf("error restoring VM user data: %s", err)
	}
	return nil
}

func restoreVmUserData(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return restoreVmUserDataOnHypervisor(hypervisor, vmIP, logger)
	}
}

func restoreVmUserDataOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.RestoreVmUserDataRequest{ipAddr}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.RestoreVmUserDataResponse
	err = client.RequestReply("Hypervisor.RestoreVmUserData", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
