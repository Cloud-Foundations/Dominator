package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmHostname(conn *srpc.Conn,
	request hypervisor.ChangeVmHostnameRequest,
	reply *hypervisor.ChangeVmHostnameResponse) error {
	*reply = hypervisor.ChangeVmHostnameResponse{
		errors.ErrorToString(
			t.manager.ChangeVmHostname(request.IpAddress,
				conn.GetAuthInformation(),
				request.Hostname))}
	return nil
}
