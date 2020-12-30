package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func removeIpAddressSubcommand(args []string, logger log.DebugLogger) error {
	ipAddr := net.ParseIP(args[0])
	if len(ipAddr) < 4 {
		return fmt.Errorf("invalid IP address: %s", args[0])
	}
	err := removeAddress(proto.Address{IpAddress: ipAddr}, logger)
	if err != nil {
		return fmt.Errorf("error removing IP address: %s", err)
	}
	return nil
}

func removeMacAddressSubcommand(args []string, logger log.DebugLogger) error {
	address := proto.Address{MacAddress: args[0]}
	err := removeAddress(address, logger)
	if err != nil {
		return fmt.Errorf("error removing MAC address: %s", err)
	}
	return nil
}

func removeAddress(address proto.Address, logger log.DebugLogger) error {
	address.Shrink()
	request := proto.ChangeAddressPoolRequest{
		AddressesToRemove: []proto.Address{address}}
	var reply proto.ChangeAddressPoolResponse
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.RequestReply("Hypervisor.ChangeAddressPool", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
