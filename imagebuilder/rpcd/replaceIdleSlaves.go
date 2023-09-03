package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) ReplaceIdleSlaves(conn *srpc.Conn,
	request proto.ReplaceIdleSlavesRequest,
	reply *proto.ReplaceIdleSlavesResponse) error {
	reply.Error = errors.ErrorToString(
		t.builder.ReplaceIdleSlaves(request.ImmediateGetNew))
	return nil
}
