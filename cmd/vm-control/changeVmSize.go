package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func changeVmCPUsSubcommand(args []string, logger log.DebugLogger) error {
	req := proto.ChangeVmSizeRequest{MilliCPUs: *milliCPUs}
	if err := changeVmSize(args[0], req, logger); err != nil {
		return fmt.Errorf("error changing VM CPUs: %s", err)
	}
	return nil
}

func changeVmMemorySubcommand(args []string, logger log.DebugLogger) error {
	req := proto.ChangeVmSizeRequest{MemoryInMiB: uint64(memory >> 20)}
	if err := changeVmSize(args[0], req, logger); err != nil {
		return fmt.Errorf("error changing VM memory: %s", err)
	}
	return nil
}

func changeVmVirtualCPUsSubcommand(args []string,
	logger log.DebugLogger) error {
	req := proto.ChangeVmSizeRequest{VirtualCPUs: *virtualCPUs}
	if err := changeVmSize(args[0], req, logger); err != nil {
		return fmt.Errorf("error changing VM memory: %s", err)
	}
	return nil
}

func changeVmSize(vmHostname string, req proto.ChangeVmSizeRequest,
	logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmSizeOnHypervisor(hypervisor, vmIP, req, logger)
	}
}

func changeVmSizeOnHypervisor(hypervisor string, ipAddr net.IP,
	req proto.ChangeVmSizeRequest, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	req.IpAddress = ipAddr
	return hyperclient.ChangeVmSize(client, req)
}
