package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) DeleteDirectory(conn *srpc.Conn,
	request imageserver.DeleteDirectoryRequest,
	reply *imageserver.DeleteDirectoryResponse) error {
	username := conn.Username()
	if err := t.checkMutability(); err != nil {
		return err
	}
	if username == "" {
		t.logger.Printf("DeleteDirectory(%s)\n", request.DirectoryName)
	} else {
		t.logger.Printf("DeleteDirectory(%s) by %s\n",
			request.DirectoryName, username)
	}
	reply.Error = errors.ErrorToString(
		t.imageDataBase.DeleteDirectory(request.DirectoryName,
			conn.GetAuthInformation()))
	return nil
}
