package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) ChangeVmNumNetworkQueues(conn *srpc.Conn,
	request hypervisor.ChangeVmNumNetworkQueuesRequest,
	reply *hypervisor.ChangeVmNumNetworkQueuesResponse) error {
	response := hypervisor.ChangeVmNumNetworkQueuesResponse{
		errors.ErrorToString(
			t.manager.ChangeVmNumNetworkQueues(request.IpAddress,
				conn.GetAuthInformation(),
				request.NumQueuesPerInterface))}
	*reply = response
	return nil
}
