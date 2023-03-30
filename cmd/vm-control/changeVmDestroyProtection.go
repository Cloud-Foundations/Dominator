package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func changeVmDestroyProtectionSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmDestroyProtection(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM destroy protection: %s", err)
	}
	return nil
}

func changeVmDestroyProtection(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmDestroyProtectionOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmDestroyProtectionOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.ChangeVmDestroyProtectionRequest{
		DestroyProtection: *destroyProtection,
		IpAddress:         ipAddr,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.ChangeVmOwnerUsersResponse
	err = client.RequestReply("Hypervisor.ChangeVmDestroyProtection",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
