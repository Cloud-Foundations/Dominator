package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) SetDisabledState(conn *srpc.Conn,
	request hypervisor.SetDisabledStateRequest,
	reply *hypervisor.SetDisabledStateResponse) error {
	t.logger.Printf("SetDisabledState(%v) by %s\n",
		request.Disable, conn.Username())
	response := hypervisor.SetDisabledStateResponse{
		errors.ErrorToString(t.manager.SetDisabledState(request.Disable))}
	*reply = response
	return nil
}
