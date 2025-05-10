package main

import (
	"fmt"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func dialFleetManager() (*srpc.Client, error) {
	if *fleetManagerHostname == "" {
		return nil, fmt.Errorf("no Fleet Manager specified")
	}
	clientName := fmt.Sprintf("%s:%d", *fleetManagerHostname,
		*fleetManagerPortNum)
	return srpc.DialHTTPWithDialer("tcp", clientName, rrDialer)
}

func dialHypervisor(address string) (*srpc.Client, error) {
	return srpc.DialHTTP("tcp", address, 0)
}

func findHypervisor(vmIpAddr net.IP) (string, error) {
	if *hypervisorHostname != "" {
		return fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			nil
	} else if *fleetManagerHostname != "" {
		cm := fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)
		client, err := srpc.DialHTTPWithDialer("tcp", cm, rrDialer)
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

func getFleetManagerClientResource() (*srpc.ClientResource, error) {
	if *fleetManagerHostname == "" {
		return nil, fmt.Errorf("no Fleet Manager specified")
	}
	return srpc.NewClientResource("tcp",
			fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)),
		nil
}

func lookupIP(vmHostname string) (net.IP, error) {
	if ips, err := net.LookupIP(vmHostname); err != nil {
		return nil, err
	} else if len(ips) != 1 {
		return nil, fmt.Errorf("num IPs: %d != 1", len(ips))
	} else {
		return ips[0], nil
	}
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
