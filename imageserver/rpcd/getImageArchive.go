package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetImageArchive(conn *srpc.Conn,
	request imageserver.GetImageArchiveRequest,
	reply *imageserver.GetImageArchiveResponse) error {
	var response imageserver.GetImageArchiveResponse
	if t.replicationMaster == "" {
		archiveData, err := t.imageDataBase.GetImageArchive(request.ImageName)
		response.ArchiveData = archiveData
		response.Error = errors.ErrorToString(err)
	} else {
		response.ReplicationMaster = t.replicationMaster
	}
	*reply = response
	return nil
}
