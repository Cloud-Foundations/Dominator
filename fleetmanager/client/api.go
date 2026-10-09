package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func Allocate(client srpc.ClientI, request proto.AllocateRequest) (
	proto.AllocateResponse, error) {
	return allocate(client, request)
}

func CancelAllocation(client srpc.ClientI, requestId proto.RequestId) error {
	return cancelAllocation(client, requestId)
}

func PowerOnMachine(client srpc.ClientI, hostname string) error {
	return powerOnMachine(client, hostname)
}
