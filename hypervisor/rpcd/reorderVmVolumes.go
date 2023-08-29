package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ReorderVmVolumes(conn *srpc.Conn,
	request hypervisor.ReorderVmVolumesRequest,
	reply *hypervisor.ReorderVmVolumesResponse) error {
	*reply = hypervisor.ReorderVmVolumesResponse{
		errors.ErrorToString(t.manager.ReorderVmVolumes(request.IpAddress,
			conn.GetAuthInformation(), request.AccessToken,
			request.VolumeIndices))}
	return nil
}
