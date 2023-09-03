package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ReplaceVmCredentials(conn *srpc.Conn,
	request hypervisor.ReplaceVmCredentialsRequest,
	reply *hypervisor.ReplaceVmCredentialsResponse) error {
	response := hypervisor.ReplaceVmCredentialsResponse{
		errors.ErrorToString(t.manager.ReplaceVmCredentials(request,
			conn.GetAuthInformation()))}
	*reply = response
	return nil
}
