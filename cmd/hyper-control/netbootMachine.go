package main

import (
	"fmt"
	"io/ioutil"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func netbootMachineSubcommand(args []string, logger log.DebugLogger) error {
	var hostname string
	if len(args) > 2 {
		hostname = args[2]
	}
	err := netbootMachine(args[0], args[1], hostname, logger)
	if err != nil {
		return fmt.Errorf("error netbooting machine: %s", err)
	}
	return nil
}

func netbootMachine(macAddr, ipAddr, hostname string,
	logger log.DebugLogger) error {
	request := proto.NetbootMachineRequest{
		Address: proto.Address{
			MacAddress: macAddr,
			IpAddress:  net.ParseIP(ipAddr),
		},
		Files: make(map[string][]byte,
			len(netbootFiles)),
		FilesExpiration:              *netbootFilesTimeout,
		Hostname:                     hostname,
		NumAcknowledgementsToWaitFor: *numAcknowledgementsToWaitFor,
		OfferExpiration:              *offerTimeout,
		WaitTimeout:                  *netbootTimeout,
	}
	for tftpFilename, localFilename := range netbootFiles {
		if data, err := ioutil.ReadFile(localFilename); err != nil {
			return err
		} else {
			request.Files[tftpFilename] = data
		}
	}
	var reply proto.NetbootMachineResponse
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.RequestReply("Hypervisor.NetbootMachine", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
