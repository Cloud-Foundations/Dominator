package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (t *rpcType) ListSubs(conn *srpc.Conn,
	request dominator.ListSubsRequest,
	reply *dominator.ListSubsResponse) error {
	hostnames, err := t.herd.ListSubs(request)
	response := dominator.ListSubsResponse{
		Error:     errors.ErrorToString(err),
		Hostnames: hostnames,
	}
	*reply = response
	return nil
}
