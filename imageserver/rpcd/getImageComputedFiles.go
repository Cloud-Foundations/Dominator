package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetImageComputedFiles(conn *srpc.Conn,
	request imageserver.GetImageComputedFilesRequest,
	reply *imageserver.GetImageComputedFilesResponse) error {
	computedFiles, ok := t.imageDataBase.GetImageComputedFiles(
		request.ImageName)
	reply.ComputedFiles = computedFiles
	reply.ImageExists = ok
	return nil
}
