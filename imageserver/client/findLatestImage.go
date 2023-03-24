package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func findLatestImage(client *srpc.Client,
	request imageserver.FindLatestImageRequest) (string, error) {
	var reply imageserver.FindLatestImageResponse
	err := client.RequestReply("ImageServer.FindLatestImage", request, &reply)
	if err == nil {
		err = errors.New(reply.Error)
	}
	if err != nil {
		return "", err
	}
	return reply.ImageName, nil
}
