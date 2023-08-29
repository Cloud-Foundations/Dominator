package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmVolumeSize(conn *srpc.Conn,
	request hypervisor.ChangeVmVolumeSizeRequest,
	reply *hypervisor.ChangeVmVolumeSizeResponse) error {
	*reply = hypervisor.ChangeVmVolumeSizeResponse{
		errors.ErrorToString(t.manager.ChangeVmVolumeSize(request.IpAddress,
			conn.GetAuthInformation(), request.VolumeIndex,
			request.VolumeSize))}
	return nil
}
