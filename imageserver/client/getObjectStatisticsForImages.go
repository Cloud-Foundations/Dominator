package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getObjectStatisticsForImages(client srpc.ClientI,
	request proto.GetObjectStatisticsForImagesRequest) (
	proto.GetObjectStatisticsForImagesResponse, error) {
	var reply imageserver.GetObjectStatisticsForImagesResponse
	err := client.RequestReply("ImageServer.GetObjectStatisticsForImages",
		request, &reply)
	if err != nil {
		return proto.GetObjectStatisticsForImagesResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.GetObjectStatisticsForImagesResponse{}, err
	}
	return reply, nil
}
