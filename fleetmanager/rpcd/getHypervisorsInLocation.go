package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) GetHypervisorsInLocation(conn *srpc.Conn,
	request proto.GetHypervisorsInLocationRequest,
	reply *proto.GetHypervisorsInLocationResponse) error {
	response, err := t.hypervisorsManager.GetHypervisorsInLocation(request)
	if err == nil {
		*reply = response
	} else {
		*reply = proto.GetHypervisorsInLocationResponse{
			Error: errors.ErrorToString(err),
		}
	}
	return nil
}
