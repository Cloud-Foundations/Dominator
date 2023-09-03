package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) ListHypervisorsInLocation(conn *srpc.Conn,
	request proto.ListHypervisorsInLocationRequest,
	reply *proto.ListHypervisorsInLocationResponse) error {
	response, err := t.hypervisorsManager.ListHypervisorsInLocation(request)
	if err == nil {
		*reply = response
	} else {
		*reply = proto.ListHypervisorsInLocationResponse{
			Error: errors.ErrorToString(err),
		}
	}
	return nil
}
