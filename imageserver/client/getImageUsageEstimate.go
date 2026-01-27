package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getImageUsageEstimate(client srpc.ClientI, name string) (
	uint64, bool, error) {
	request := imageserver.GetImageUsageEstimateRequest{ImageName: name}
	var reply imageserver.GetImageUsageEstimateResponse
	err := client.RequestReply("ImageServer.GetImageUsageEstimate",
		request, &reply)
	if err != nil {
		return 0, false, err
	}
	return reply.UsageEstimate, reply.ImageExists, nil
}
