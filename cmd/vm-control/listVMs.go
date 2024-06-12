package main

import (
	"fmt"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func listVMsSubcommand(args []string, logger log.DebugLogger) error {
	if err := listVMs(logger); err != nil {
		return fmt.Errorf("error listing VMs: %s", err)
	}
	return nil
}

func listVMs(logger log.DebugLogger) error {
	if *hypervisorHostname != "" {
		return listVMsOnHypervisor(
			fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			logger)
	}
	if *fleetManagerHostname != "" {
		fleetManager := fmt.Sprintf("%s:%d",
			*fleetManagerHostname, *fleetManagerPortNum)
		return listVMsByLocation(fleetManager, *location, logger)
	}
	return listVMsOnHypervisor(fmt.Sprintf("localhost:%d", *hypervisorPortNum),
		logger)
}

func listVMsByLocation(fleetManager string, location string,
	logger log.DebugLogger) error {
	client, err := dialFleetManager(fleetManager)
	if err != nil {
		return err
	}
	defer client.Close()
	conn, err := client.Call("FleetManager.ListVMsInLocation")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := fm_proto.ListVMsInLocationRequest{
		HypervisorTagsToMatch: hypervisorTagsToMatch,
		Location:              location,
		OwnerGroups:           ownerGroups,
		OwnerUsers:            ownerUsers,
		VmTagsToMatch:         vmTagsToMatch,
	}
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var addresses []string
	for {
		var reply fm_proto.ListVMsInLocationResponse
		if err := conn.Decode(&reply); err != nil {
			return err
		}
		if err := errors.New(reply.Error); err != nil {
			return err
		}
		if len(reply.IpAddresses) < 1 {
			break
		}
		for _, ipAddress := range reply.IpAddresses {
			addresses = append(addresses, ipAddress.String())
		}
	}
	verstr.Sort(addresses)
	for _, address := range addresses {
		if _, err := fmt.Println(address); err != nil {
			return err
		}
	}
	return nil
}

func listVMsOnHypervisor(hypervisor string, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	ipAddresses, err := hyperclient.ListVMs(client,
		hyper_proto.ListVMsRequest{
			OwnerGroups:   ownerGroups,
			OwnerUsers:    ownerUsers,
			Sort:          true,
			VmTagsToMatch: vmTagsToMatch,
		})
	if err != nil {
		return err
	}
	for _, ipAddress := range ipAddresses {
		if _, err := fmt.Println(ipAddress); err != nil {
			return err
		}
	}
	return nil
}
