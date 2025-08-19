package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func getIpInfoSubcommand(args []string, logger log.DebugLogger) error {
	if err := getIpInfo(args[0], logger); err != nil {
		return fmt.Errorf("error getting IP info: %s", err)
	}
	return nil
}

func getIpInfo(hostname string, logger log.DebugLogger) error {
	fleetManager := fmt.Sprintf("%s:%d",
		*fleetManagerHostname, *fleetManagerPortNum)
	client, err := dialFleetManager(fleetManager)
	if err != nil {
		return err
	}
	defer client.Close()
	ipAddr, err := lookupIP(hostname)
	if err != nil {
		return err
	}
	if hostname != ipAddr.String() {
		hostname += "(" + ipAddr.String() + ")"
	}
	request := proto.GetIpInfoRequest{ipAddr}
	var reply proto.GetIpInfoResponse
	err = client.RequestReply("FleetManager.GetIpInfo", request,
		&reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	if reply.HypervisorAddress == "" {
		fmt.Printf("%s is not registered with any Hypervisor\n", hostname)
	} else if reply.VM != nil {
		fmt.Printf("%s is allocated to VM %s(%s) on Hypervisor: %s\n",
			hostname, reply.VM.Address.IpAddress, reply.VM.Hostname,
			reply.HypervisorAddress)
	} else {
		fmt.Printf("%s is registered and free on Hypervisor: %s\n",
			hostname, reply.HypervisorAddress)
	}
	return nil
}
