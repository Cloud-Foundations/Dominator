package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) GetIpInfo(conn *srpc.Conn,
	request proto.GetIpInfoRequest,
	reply *proto.GetIpInfoResponse) error {
	ipAddr := request.IpAddress
	if response, err := t.hypervisorsManager.GetIpInfo(ipAddr); err != nil {
		*reply = proto.GetIpInfoResponse{
			Error: errors.ErrorToString(err),
		}
	} else {
		*reply = response
	}
	return nil
}
