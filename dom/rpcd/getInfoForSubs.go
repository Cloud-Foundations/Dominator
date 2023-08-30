package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (t *rpcType) GetInfoForSubs(conn *srpc.Conn,
	request dominator.GetInfoForSubsRequest,
	reply *dominator.GetInfoForSubsResponse) error {
	subs, err := t.herd.GetInfoForSubs(request)
	response := dominator.GetInfoForSubsResponse{
		Error: errors.ErrorToString(err),
		Subs:  subs,
	}
	*reply = response
	return nil
}
