package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetCapacity(conn *srpc.Conn,
	request hypervisor.GetCapacityRequest,
	reply *hypervisor.GetCapacityResponse) error {
	*reply = t.manager.GetCapacity()
	return nil
}
