package rpcd

import (
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetVmLastPatchLog(conn *srpc.Conn) error {
	var request proto.GetVmLastPatchLogRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	rc, length, patchTime, err := t.manager.GetVmLastPatchLog(request.IpAddress)
	if err != nil {
		if os.IsNotExist(err) {
			return conn.Encode(proto.GetVmLastPatchLogResponse{})
		}
		return conn.Encode(proto.GetVmLastPatchLogResponse{Error: err.Error()})
	}
	response := proto.GetVmLastPatchLogResponse{
		Length:    length,
		PatchTime: patchTime,
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	_, err = io.CopyN(conn, rc, int64(length))
	return err
}
