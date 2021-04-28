package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) GetDirectedGraph(conn *srpc.Conn,
	request proto.GetDirectedGraphRequest,
	reply *proto.GetDirectedGraphResponse) error {
	data, err := t.builder.GetDirectedGraph(request)
	reply.GraphvizDot = data
	reply.Error = errors.ErrorToString(err)
	return nil
}
