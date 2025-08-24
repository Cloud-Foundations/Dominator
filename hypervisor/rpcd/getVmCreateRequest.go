package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetVmCreateRequest(conn *srpc.Conn,
	request hypervisor.GetVmCreateRequestRequest,
	reply *hypervisor.GetVmCreateRequestResponse) error {
	createRequest, err := t.manager.GetVmCreateRequest(request.IpAddress,
		conn.GetAuthInformation())
	if err != nil {
		*reply = hypervisor.GetVmCreateRequestResponse{Error: err.Error()}
	} else {
		*reply = hypervisor.GetVmCreateRequestResponse{
			CreateVmRequest: *createRequest,
		}
	}
	return nil
}
