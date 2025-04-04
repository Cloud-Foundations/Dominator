package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getImageArchive(client srpc.ClientI, name string) (
	imageserver.GetImageArchiveResponse, error) {
	request := imageserver.GetImageArchiveRequest{ImageName: name}
	var reply imageserver.GetImageArchiveResponse
	err := client.RequestReply("ImageServer.GetImageArchive", request, &reply)
	if err != nil {
		return imageserver.GetImageArchiveResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return imageserver.GetImageArchiveResponse{}, err
	}
	return reply, nil
}
