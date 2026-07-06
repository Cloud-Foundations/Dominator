package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func (t *srpcType) CancelAllocation(conn *srpc.Conn,
	request proto.CancelAllocationRequest,
	reply *proto.CancelAllocationResponse) error {
	err := t.allocationManager.CancelAllocation(conn.GetAuthInformation(),
		request.RequestId)
	if err != nil {
		*reply = proto.CancelAllocationResponse{
			Error: errors.ErrorToString(err)}
	}
	return nil
}
