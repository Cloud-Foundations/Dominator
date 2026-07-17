package client

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func allocate(client srpc.ClientI, request proto.AllocateRequest) (
	proto.AllocateResponse, error) {
	if err := request.CheckValid(); err != nil {
		return proto.AllocateResponse{},
			fmt.Errorf("local validation failure: %s", err)
	}
	var reply proto.AllocateResponse
	err := client.RequestReply("FleetManager.Allocate", request, &reply)
	if err != nil {
		return proto.AllocateResponse{}, err
	}
	if reply.Error != "" {
		return proto.AllocateResponse{}, errors.New(reply.Error)
	}
	return reply, nil
}

func cancelAllocation(client srpc.ClientI, requestId proto.RequestId) error {
	request := proto.CancelAllocationRequest{RequestId: requestId}
	var reply proto.CancelAllocationResponse
	err := client.RequestReply("FleetManager.CancelAllocation",
		request, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	return nil
}

func powerOnMachine(client srpc.ClientI, hostname string) error {
	request := proto.PowerOnMachineRequest{Hostname: hostname}
	var reply proto.PowerOnMachineResponse
	err := client.RequestReply("FleetManager.PowerOnMachine", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
