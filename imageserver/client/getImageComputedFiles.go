package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getImageComputedFiles(client srpc.ClientI, name string) (
	[]filesystem.ComputedFile, bool, error) {
	request := imageserver.GetImageComputedFilesRequest{ImageName: name}
	var reply imageserver.GetImageComputedFilesResponse
	err := client.RequestReply("ImageServer.GetImageComputedFiles",
		request, &reply)
	if err != nil {
		return nil, false, err
	}
	return reply.ComputedFiles, reply.ImageExists, nil
}
