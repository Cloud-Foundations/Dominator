package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func changeVmSubnetSubcommand(args []string, logger log.DebugLogger) error {
	if err := changeVmSubnet(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM subnet: %s", err)
	}
	return nil
}

func changeVmSubnet(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmSubnetOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmSubnetOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	if *subnetId == "" {
		return fmt.Errorf("subnetId not specified")
	}
	req := proto.ChangeVmSubnetRequest{
		IpAddress: ipAddr,
		SubnetId:  *subnetId,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	reply, err := hyperclient.ChangeVmSubnet(client, req)
	if err != nil {
		return err
	}
	fmt.Println(reply.NewIpAddress)
	return nil
}
