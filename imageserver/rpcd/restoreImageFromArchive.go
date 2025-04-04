package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) RestoreImageFromArchive(conn *srpc.Conn,
	request imageserver.RestoreImageFromArchiveRequest,
	reply *imageserver.RestoreImageFromArchiveResponse) error {
	if t.replicationMaster != "" {
		*reply = imageserver.RestoreImageFromArchiveResponse{
			ReplicationMaster: t.replicationMaster,
		}
		return nil
	}
	err := t.imageDataBase.RestoreImageFromArchive(request,
		conn.GetAuthInformation())
	*reply = imageserver.RestoreImageFromArchiveResponse{
		Error: errors.ErrorToString(err),
	}
	return nil
}
