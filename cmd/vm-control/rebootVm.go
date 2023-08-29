package main

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func rebootVmSubcommand(args []string, logger log.DebugLogger) error {
	if err := rebootVm(args[0], logger); err != nil {
		return fmt.Errorf("error rebooting VM: %s", err)
	}
	return nil
}

func rebootVm(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return rebootVmOnHypervisor(hypervisor, vmIP, logger)
	}
}

func rebootVmOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	request := proto.RebootVmRequest{
		DhcpTimeout: *dhcpTimeout,
		IpAddress:   ipAddr,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply proto.RebootVmResponse
	err = client.RequestReply("Hypervisor.RebootVm", request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	if reply.DhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	return maybeWatchVm(client, hypervisor, ipAddr, logger)
}
