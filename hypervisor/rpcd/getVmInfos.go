package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetVmInfos(conn *srpc.Conn,
	request hypervisor.GetVmInfosRequest,
	reply *hypervisor.GetVmInfosResponse) error {
	vmInfos, err := t.manager.GetVmInfos(request)
	*reply = hypervisor.GetVmInfosResponse{
		Error:   errors.ErrorToString(err),
		VmInfos: vmInfos,
	}
	return nil
}
