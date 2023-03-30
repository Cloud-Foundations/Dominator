package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) GetDirectedGraph(conn *srpc.Conn,
	request proto.GetDirectedGraphRequest,
	reply *proto.GetDirectedGraphResponse) error {
	if result, err := t.builder.GetDirectedGraph(request); err != nil {
		reply.Error = errors.ErrorToString(err)
	} else {
		reply.GetDirectedGraphResult = result
	}
	return nil
}
