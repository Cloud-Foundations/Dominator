package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmOwnerGroups(conn *srpc.Conn,
	request hypervisor.ChangeVmOwnerGroupsRequest,
	reply *hypervisor.ChangeVmOwnerGroupsResponse) error {
	response := hypervisor.ChangeVmOwnerGroupsResponse{
		errors.ErrorToString(
			t.manager.ChangeVmOwnerGroups(request.IpAddress,
				conn.GetAuthInformation(),
				request.OwnerGroups))}
	*reply = response
	return nil
}
