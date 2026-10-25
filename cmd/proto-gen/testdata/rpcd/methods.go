package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type srpcType struct{}

// @grpc
func (t *srpcType) GetVmInfo(conn *srpc.Conn,
	request hypervisor.GetVmInfoRequest,
	reply *hypervisor.GetVmInfoResponse) error {
	return nil
}

// ListVms returns the VM IP addresses.
// @grpc no-rest
func (t *srpcType) ListVMs(conn *srpc.Conn,
	request hypervisor.ListVMsRequest,
	reply *hypervisor.ListVMsResponse) error {
	return nil
}

// StartVm is not annotated, so it is not exported.
func (t *srpcType) StartVm(conn *srpc.Conn,
	request hypervisor.StartVmRequest,
	reply *hypervisor.StartVmResponse) error {
	return nil
}
