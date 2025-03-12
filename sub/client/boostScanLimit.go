package client

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
)

func boostScanLimit(client *srpc.Client) error {
	request := sub.BoostScanLimitRequest{}
	var reply sub.BoostScanLimitResponse
	return client.RequestReply("Subd.BoostScanLimit", request, &reply)
}
