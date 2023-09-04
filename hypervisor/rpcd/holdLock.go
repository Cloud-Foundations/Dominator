package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) HoldLock(conn *srpc.Conn, request hypervisor.HoldLockRequest,
	reply *hypervisor.HoldLockResponse) error {
	if request.WriteLock {
		t.logger.Printf("HoldLock(%s) by %s for writing\n",
			format.Duration(request.Timeout), conn.Username())
	} else {
		t.logger.Printf("HoldLock(%s) by %s for reading\n",
			format.Duration(request.Timeout), conn.Username())
	}
	response := hypervisor.HoldLockResponse{
		errors.ErrorToString(t.manager.HoldLock(request.Timeout,
			request.WriteLock))}
	*reply = response
	return nil
}
