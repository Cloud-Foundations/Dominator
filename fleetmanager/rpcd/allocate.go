package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) Allocate(conn *srpc.Conn,
	request proto.AllocateRequest, reply *proto.AllocateResponse) error {
	response, err := t.allocationManager.Allocate(conn.GetAuthInformation(),
		request)
	if err != nil {
		*reply = proto.AllocateResponse{
			Error: errors.ErrorToString(err),
		}
	} else {
		*reply = response
	}
	return nil
}
