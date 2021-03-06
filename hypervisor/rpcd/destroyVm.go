package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) DestroyVm(conn *srpc.Conn,
	request hypervisor.DestroyVmRequest,
	reply *hypervisor.DestroyVmResponse) error {
	response := hypervisor.DestroyVmResponse{
		errors.ErrorToString(t.manager.DestroyVm(request.IpAddress,
			conn.GetAuthInformation(), request.AccessToken))}
	*reply = response
	return nil
}
