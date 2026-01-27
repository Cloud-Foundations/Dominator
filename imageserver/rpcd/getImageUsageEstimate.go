package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func (t *srpcType) GetImageUsageEstimate(conn *srpc.Conn,
	request imageserver.GetImageUsageEstimateRequest,
	reply *imageserver.GetImageUsageEstimateResponse) error {
	usage, found := t.imageDataBase.GetImageUsageEstimate(request.ImageName)
	reply.ImageExists = found
	reply.UsageEstimate = usage
	return nil
}
