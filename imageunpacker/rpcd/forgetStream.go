package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func (t *srpcType) ForgetStream(conn *srpc.Conn,
	request proto.ForgetStreamRequest,
	reply *proto.ForgetStreamResponse) error {
	return t.unpacker.ForgetStream(request.StreamName)
}
