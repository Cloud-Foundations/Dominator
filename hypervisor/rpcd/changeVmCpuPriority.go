package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmCpuPriority(conn *srpc.Conn,
	request hypervisor.ChangeVmCpuPriorityRequest,
	reply *hypervisor.ChangeVmCpuPriorityResponse) error {
	*reply = hypervisor.ChangeVmCpuPriorityResponse{
		errors.ErrorToString(
			t.manager.ChangeVmCpuPriority(request.IpAddress,
				conn.GetAuthInformation(),
				request.CpuPriority))}
	return nil
}
