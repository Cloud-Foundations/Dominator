package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imaginator"
)

func (t *srpcType) StartAutoBuilds(conn *srpc.Conn,
	request proto.StartAutoBuildsRequest,
	reply *proto.StartAutoBuildsResponse) error {
	if authInfo := conn.GetAuthInformation(); authInfo != nil {
		t.logger.Printf("StartAutoBuilds(%s)\n", authInfo.Username)
	}
	if err := t.builder.StartAutoBuilds(request); err != nil {
		reply.Error = err.Error()
	}
	return nil
}
