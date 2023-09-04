package rpcd

import (
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (t *srpcType) GetMachineInfo(conn *srpc.Conn,
	request fm_proto.GetMachineInfoRequest,
	reply *fm_proto.GetMachineInfoResponse) error {
	if response, err := t.getMachineInfo(request); err != nil {
		*reply = fm_proto.GetMachineInfoResponse{
			Error: errors.ErrorToString(err)}
	} else {
		*reply = response
	}
	return nil
}

func (t *srpcType) getMachineInfo(request fm_proto.GetMachineInfoRequest) (
	fm_proto.GetMachineInfoResponse, error) {
	topology, err := t.hypervisorsManager.GetTopology()
	if err != nil {
		return fm_proto.GetMachineInfoResponse{}, err
	}
	location, err := topology.GetLocationOfMachine(request.Hostname)
	if err != nil {
		return fm_proto.GetMachineInfoResponse{}, err
	}
	machine, err := t.hypervisorsManager.GetMachineInfo(request)
	if err != nil {
		return fm_proto.GetMachineInfoResponse{}, err
	}
	tSubnets, err := topology.GetSubnetsForMachine(request.Hostname)
	if err != nil {
		return fm_proto.GetMachineInfoResponse{}, err
	}
	subnets := make([]*hyper_proto.Subnet, 0, len(tSubnets))
	for _, tSubnet := range tSubnets {
		subnets = append(subnets, &tSubnet.Subnet)
	}
	return fm_proto.GetMachineInfoResponse{
		Location: location,
		Machine:  machine,
		Subnets:  subnets,
	}, nil
}
