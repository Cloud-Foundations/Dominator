package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ListVmVirtualiserLogFiles(conn *srpc.Conn,
	request hypervisor.ListVmVirtualiserLogFilesRequest,
	reply *hypervisor.ListVmVirtualiserLogFilesResponse) error {
	filenames, lengths, err :=
		t.manager.ListVmVirtualiserLogFiles(request.IpAddress)
	*reply = hypervisor.ListVmVirtualiserLogFilesResponse{
		Error:     errors.ErrorToString(err),
		Filenames: filenames,
		Lengths:   lengths,
	}
	return nil
}
