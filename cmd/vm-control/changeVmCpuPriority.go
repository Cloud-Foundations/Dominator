package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func changeVmCpuPrioritySubcommand(args []string,
	logger log.DebugLogger) error {
	if err := changeVmCpuPriority(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM CPU priority: %s", err)
	}
	return nil
}

func changeVmCpuPriority(vmHostname string,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmCpuPriorityOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmCpuPriorityOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.ChangeVmCpuPriorityRequest{
		CpuPriority: *cpuPriority,
		IpAddress:   ipAddr,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.ChangeVmCpuPriorityResponse
	err = client.RequestReply("Hypervisor.ChangeVmCpuPriority",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
