package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmVolumeInterfaces(conn *srpc.Conn,
	request hypervisor.ChangeVmVolumeInterfacesRequest,
	reply *hypervisor.ChangeVmVolumeInterfacesResponse) error {
	*reply = hypervisor.ChangeVmVolumeInterfacesResponse{
		errors.ErrorToString(
			t.manager.ChangeVmVolumeInterfaces(request.IpAddress,
				conn.GetAuthInformation(), request.Interfaces))}
	return nil
}
