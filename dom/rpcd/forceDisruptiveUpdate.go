package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/dominator"
)

func (t *rpcType) ForceDisruptiveUpdate(conn *srpc.Conn,
	request dominator.ForceDisruptiveUpdateRequest,
	reply *dominator.ForceDisruptiveUpdateResponse) error {
	if conn.Username() == "" {
		t.logger.Printf("ForceDisruptiveUpdate(%s)\n", request.Hostname)
	} else {
		t.logger.Printf("ForceDisruptiveUpdate(%s): by %s\n",
			request.Hostname, conn.Username())
	}
	return t.herd.ForceDisruptiveUpdate(request.Hostname,
		conn.GetAuthInformation())
}
