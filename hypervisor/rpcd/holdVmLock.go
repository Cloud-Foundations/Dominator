package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) HoldVmLock(conn *srpc.Conn,
	request hypervisor.HoldVmLockRequest,
	reply *hypervisor.HoldVmLockResponse) error {
	response := hypervisor.HoldVmLockResponse{
		errors.ErrorToString(t.manager.HoldVmLock(request.IpAddress,
			request.Timeout, request.WriteLock, conn.GetAuthInformation()))}
	*reply = response
	return nil
}
