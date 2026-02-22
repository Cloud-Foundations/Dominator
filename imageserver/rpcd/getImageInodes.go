package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetImageInodes(conn *srpc.Conn,
	request imageserver.GetImageInodesRequest,
	reply *imageserver.GetImageInodesResponse) error {
	response, err := t.imageDataBase.GetImageInodes(request.ImageName,
		request.Filenames)
	if err != nil {
		reply.Error = err.Error()
	} else {
		*reply = response
	}
	return nil
}
