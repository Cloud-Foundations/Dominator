package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmVolumeStorageIndex(conn *srpc.Conn,
	request hypervisor.ChangeVmVolumeStorageIndexRequest,
	reply *hypervisor.ChangeVmVolumeStorageIndexResponse) error {
	*reply = hypervisor.ChangeVmVolumeStorageIndexResponse{
		errors.ErrorToString(t.manager.ChangeVmVolumeStorageIndex(
			request.IpAddress, conn.GetAuthInformation(), request.StorageIndex,
			request.VolumeIndex))}
	return nil
}
