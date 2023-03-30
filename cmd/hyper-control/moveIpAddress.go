package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func moveIpAddressSubcommand(args []string, logger log.DebugLogger) error {
	if err := moveIpAddress(args[0], logger); err != nil {
		return fmt.Errorf("error moving IP address: %s", err)
	}
	return nil
}

func moveIpAddress(addr string, logger log.DebugLogger) error {
	ipAddr := net.ParseIP(addr)
	if len(ipAddr) < 4 {
		return fmt.Errorf("invalid IP address: %s", addr)
	}
	request := proto.MoveIpAddressesRequest{
		HypervisorHostname: *hypervisorHostname,
		IpAddresses:        []net.IP{ipAddr},
	}
	var reply proto.MoveIpAddressesResponse
	clientName := fmt.Sprintf("%s:%d",
		*fleetManagerHostname, *fleetManagerPortNum)
	client, err := srpc.DialHTTPWithDialer("tcp", clientName, rrDialer)
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.RequestReply("FleetManager.MoveIpAddresses", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
