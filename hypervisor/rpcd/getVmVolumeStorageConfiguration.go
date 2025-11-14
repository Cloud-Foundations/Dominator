package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetVmVolumeStorageConfiguration(conn *srpc.Conn,
	request hypervisor.GetVmVolumeStorageConfigurationRequest,
	reply *hypervisor.GetVmVolumeStorageConfigurationResponse) error {
	response, err := t.manager.GetVmVolumeStorageConfiguration(
		request.IpAddress, conn.GetAuthInformation(), nil)
	response.Error = errors.ErrorToString(err)
	*reply = response
	return nil
}
