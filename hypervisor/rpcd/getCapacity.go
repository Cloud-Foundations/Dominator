package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetCapacity(conn *srpc.Conn,
	request hypervisor.GetCapacityRequest,
	reply *hypervisor.GetCapacityResponse) error {
	response, err := t.manager.GetCapacity()
	if err != nil {
		*reply = hypervisor.GetCapacityResponse{Error: err.Error()}
	} else {
		*reply = response
	}
	return nil
}
