package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) TruncateVmVirtualiserLogFile(conn *srpc.Conn,
	request hypervisor.TruncateVmVirtualiserLogFileRequest,
	reply *hypervisor.TruncateVmVirtualiserLogFileResponse) error {
	authInfo := conn.GetAuthInformation()
	err := t.manager.TruncateVmVirtualiserLogFile(request.IpAddress, authInfo,
		request.Filename)
	response := hypervisor.TruncateVmVirtualiserLogFileResponse{
		errors.ErrorToString(err)}
	*reply = response
	return nil
}
