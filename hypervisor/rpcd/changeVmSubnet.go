package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmSubnet(conn *srpc.Conn,
	request hypervisor.ChangeVmSubnetRequest,
	reply *hypervisor.ChangeVmSubnetResponse) error {
	response, err := t.manager.ChangeVmSubnet(conn.GetAuthInformation(),
		request)
	if err != nil {
		*reply = hypervisor.ChangeVmSubnetResponse{
			Error: errors.ErrorToString(err),
		}
	} else {
		*reply = *response
	}
	return nil
}
