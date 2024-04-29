package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetReplicationMaster(conn *srpc.Conn,
	request imageserver.GetReplicationMasterRequest,
	reply *imageserver.GetReplicationMasterResponse) error {
	reply.ReplicationMaster = t.replicationMaster
	return nil
}
