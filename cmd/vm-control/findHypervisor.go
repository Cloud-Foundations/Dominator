package main

import (
	"fmt"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func findHypervisor(vmIpAddr net.IP) (string, error) {
	if *hypervisorHostname != "" {
		return fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			nil
	} else if *fleetManagerHostname != "" {
		cm := fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)
		client, err := dialFleetManager(cm)
		if err != nil {
			return "", err
		}
		defer client.Close()
		return findHypervisorClient(client, vmIpAddr)
	} else {
		return fmt.Sprintf("localhost:%d", *hypervisorPortNum), nil
	}
}

func findHypervisorClient(client *srpc.Client,
	vmIpAddr net.IP) (string, error) {
	request := proto.GetHypervisorForVMRequest{vmIpAddr}
	var reply proto.GetHypervisorForVMResponse
	err := client.RequestReply("FleetManager.GetHypervisorForVM", request,
		&reply)
	if err != nil {
		return "", err
	}
	if err := errors.New(reply.Error); err != nil {
		return "", err
	}
	return reply.HypervisorAddress, nil
}

func lookupIP(vmHostname string) (net.IP, error) {
	ips, err := net.LookupIP(vmHostname)
	if err != nil {
		return nil, err
	}
	ipv4, ipv6 := splitIPs(ips)
	switch {
	case len(ipv4) == 1 && len(ipv6) <= 1:
		// Prefer IPv4 when exactly one IPv4 address is available.
		return ipv4[0], nil
	case len(ipv6) == 1 && len(ipv4) == 0:
		return ipv6[0], nil
	default:
		return nil, fmt.Errorf(
			"unable to determine a unique IP address for: %s "+
				"num IPv4: %d, num IPv6: %d",
			vmHostname, len(ipv4), len(ipv6),
		)
	}
}

func splitIPs(ips []net.IP) (ipv4, ipv6 []net.IP) {
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4 = append(ipv4, ip)
		} else if ip.To16() != nil {
			ipv6 = append(ipv6, ip)
		}
	}
	return
}

func lookupVmAndHypervisor(vmHostname string) (net.IP, string, error) {
	if vmIpAddr, err := lookupIP(vmHostname); err != nil {
		return nil, "", err
	} else if hypervisor, err := findHypervisor(vmIpAddr); err != nil {
		return nil, "", err
	} else {
		return vmIpAddr, hypervisor, nil
	}
}

func searchVmAndHypervisor(vmHostname string) (net.IP, string, error) {
	if *fleetManagerHostname == "" {
		return nil, "", fmt.Errorf("no fleet manager specified")
	}
	vmIpAddr, err := lookupIP(vmHostname)
	if err != nil {
		return nil, "", err
	}
	cm := fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)
	client, err := srpc.DialHTTP("tcp", cm, time.Second*10)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()
	if hypervisor, err := findHypervisorClient(client, vmIpAddr); err != nil {
		return nil, "", err
	} else {
		return vmIpAddr, hypervisor, nil
	}
}
