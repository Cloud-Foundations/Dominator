package rpcd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetVmVirtualiserLogFile(conn *srpc.Conn) error {
	var request proto.GetVmVirtualiserLogFileRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	rc, length, err := t.manager.GetVmVirtualiserLogFile(request.IpAddress,
		conn.GetAuthInformation(), request.Filename)
	if err != nil {
		return conn.Encode(proto.GetVmVirtualiserLogFileResponse{
			Error: err.Error(),
		})
	}
	defer rc.Close()
	response := proto.GetVmUserDataResponse{Length: length}
	if err := conn.Encode(response); err != nil {
		return err
	}
	_, err = io.CopyN(conn, rc, int64(length))
	return err
}
