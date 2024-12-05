package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ReplaceVmIdentity(conn *srpc.Conn,
	request hypervisor.ReplaceVmIdentityRequest,
	reply *hypervisor.ReplaceVmIdentityResponse) error {
	response := hypervisor.ReplaceVmIdentityResponse{
		errors.ErrorToString(t.manager.ReplaceVmIdentity(request,
			conn.GetAuthInformation()))}
	*reply = response
	return nil
}
