package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func restoreImageFromArchive(client srpc.ClientI,
	request imageserver.RestoreImageFromArchiveRequest) (
	imageserver.RestoreImageFromArchiveResponse, error) {
	var reply imageserver.RestoreImageFromArchiveResponse
	err := client.RequestReply("ImageServer.RestoreImageFromArchive", request,
		&reply)
	if err != nil {
		return imageserver.RestoreImageFromArchiveResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return imageserver.RestoreImageFromArchiveResponse{}, err
	}
	return reply, nil
}
