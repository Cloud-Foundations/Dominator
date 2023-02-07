package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func (t *srpcType) ClaimDevice(conn *srpc.Conn,
	request proto.ClaimDeviceRequest,
	reply *proto.ClaimDeviceResponse) error {
	return t.unpacker.ClaimDevice(request.DeviceId, request.DeviceName)
}
