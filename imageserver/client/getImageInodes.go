package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getImageInodes(client srpc.ClientI, imageName string,
	filenames []filesystem.Filename) (
	proto.GetImageInodesResponse, error) {
	request := proto.GetImageInodesRequest{
		ImageName: imageName,
		Filenames: filenames,
	}
	var reply proto.GetImageInodesResponse
	err := client.RequestReply("ImageServer.GetImageInodes", request, &reply)
	if err != nil {
		return proto.GetImageInodesResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.GetImageInodesResponse{}, err
	}
	return reply, nil
}
