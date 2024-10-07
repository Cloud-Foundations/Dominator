package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmMachineType(conn *srpc.Conn,
	request hypervisor.ChangeVmMachineTypeRequest,
	reply *hypervisor.ChangeVmMachineTypeResponse) error {
	*reply = hypervisor.ChangeVmMachineTypeResponse{
		errors.ErrorToString(
			t.manager.ChangeVmMachineType(request.IpAddress,
				conn.GetAuthInformation(),
				request.MachineType))}
	return nil
}
