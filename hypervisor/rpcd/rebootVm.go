package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) RebootVm(conn *srpc.Conn,
	request hypervisor.RebootVmRequest,
	reply *hypervisor.RebootVmResponse) error {
	dhcpTimedOut, err := t.manager.RebootVm(request.IpAddress,
		conn.GetAuthInformation(), request.DhcpTimeout)
	response := hypervisor.RebootVmResponse{dhcpTimedOut,
		errors.ErrorToString(err)}
	*reply = response
	return nil
}
