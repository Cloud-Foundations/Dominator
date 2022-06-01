package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) GetDependencies(conn *srpc.Conn,
	request proto.GetDependenciesRequest,
	reply *proto.GetDependenciesResponse) error {
	if result, err := t.builder.GetDependencies(request); err != nil {
		reply.Error = errors.ErrorToString(err)
	} else {
		reply.GetDependenciesResult = result
	}
	return nil
}
