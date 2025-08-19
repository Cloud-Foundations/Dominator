package hypervisors

import (
	"errors"
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (h *hypervisorType) makeProtoHypervisor(
	includeVMs bool) fm_proto.Hypervisor {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	protoHypervisor := h.Hypervisor
	if includeVMs {
		protoHypervisor.VMs = make([]hyper_proto.VmInfo, 0, len(h.vms))
		for _, vm := range h.vms {
			protoHypervisor.VMs = append(protoHypervisor.VMs, vm.VmInfo)
		}
	}
	return protoHypervisor
}

func (h *hypervisorType) getSerialNumber() string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.serialNumber
}

func (m *Manager) getLockedHypervisorByHW(macAddr string) (
	*hypervisorType, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if hypervisor, ok := m.hypervisorsByHW[macAddr]; !ok {
		return nil, errors.New("Hypervisor not found")
	} else {
		hypervisor.mutex.RLock()
		return hypervisor, nil
	}
}

func (m *Manager) getLockedHypervisorByIP(ipAddr string) (
	*hypervisorType, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if hypervisor, ok := m.hypervisorsByIP[ipAddr]; !ok {
		return nil, errors.New("Hypervisor not found")
	} else {
		hypervisor.mutex.RLock()
		return hypervisor, nil
	}
}

func (m *Manager) getLockedHypervisorBySN(serialNumber string) (
	*hypervisorType, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if hypervisor := m.hypervisorsBySN[serialNumber]; hypervisor == nil {
		return nil, errors.New("Hypervisor not found")
	} else {
		hypervisor.mutex.RLock()
		return hypervisor, nil
	}
}

func (m *Manager) getLockedHypervisor(name string,
	writeLock bool) (*hypervisorType, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if hypervisor, ok := m.hypervisors[name]; !ok {
		return nil, errors.New("Hypervisor not found")
	} else {
		if writeLock {
			hypervisor.mutex.Lock()
		} else {
			hypervisor.mutex.RLock()
		}
		return hypervisor, nil
	}
}

func (m *Manager) getHypervisorForVm(ipAddr net.IP) (string, error) {
	addr := ipAddr.String()
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if vm, ok := m.vms[addr]; !ok {
		return "", errors.New("VM not found")
	} else {
		return vm.hypervisor.Machine.Hostname, nil
	}
}

func (m *Manager) getHypervisorsInLocation(
	request fm_proto.GetHypervisorsInLocationRequest) (
	fm_proto.GetHypervisorsInLocationResponse, error) {
	showFilter := showOK
	if request.IncludeUnhealthy {
		showFilter = showConnected
	}
	hypervisors, err := m.listHypervisors(request.Location, showFilter,
		request.SubnetId, tagmatcher.New(request.HypervisorTagsToMatch, false))
	if err != nil {
		return fm_proto.GetHypervisorsInLocationResponse{}, err
	}
	protoHypervisors := make([]fm_proto.Hypervisor, 0, len(hypervisors))
	for _, hypervisor := range hypervisors {
		protoHypervisors = append(protoHypervisors,
			hypervisor.makeProtoHypervisor(request.IncludeVMs))
	}
	return fm_proto.GetHypervisorsInLocationResponse{
		Hypervisors: protoHypervisors,
	}, nil
}

func (m *Manager) getIpInfo(ipAddr net.IP) (fm_proto.GetIpInfoResponse, error) {
	addr := ipAddr.String()
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if vm, ok := m.vms[addr]; ok {
		return fm_proto.GetIpInfoResponse{
			HypervisorAddress: vm.hypervisor.Machine.Hostname,
			VM:                &vm.VmInfo,
		}, nil
	}
	hyperIP, err := m.storer.GetHypervisorForIp(ipAddr)
	if err != nil {
		return fm_proto.GetIpInfoResponse{}, err
	}
	if len(hyperIP) < 1 {
		return fm_proto.GetIpInfoResponse{}, nil
	}
	hypervisor, ok := m.hypervisorsByIP[hyperIP.String()]
	if !ok {
		return fm_proto.GetIpInfoResponse{},
			fmt.Errorf("hypervisor IP=%s unknown", hyperIP)
	}
	// TODO(rgooch): replace loop with map lookup.
	var protoVM *hyper_proto.VmInfo
	for _, vm := range hypervisor.vms {
		if protoVM != nil {
			break
		}
		for _, address := range vm.SecondaryAddresses {
			if ipAddr.Equal(address.IpAddress) {
				protoVM = &vm.VmInfo
				break
			}
		}
	}
	return fm_proto.GetIpInfoResponse{
		HypervisorAddress: fmt.Sprintf("%s:%d",
			hypervisor.Machine.Hostname, constants.HypervisorPortNumber),
		VM: protoVM,
	}, nil
}

func (m *Manager) getMachineInfo(request fm_proto.GetMachineInfoRequest) (
	fm_proto.Machine, error) {
	if !*manageHypervisors && !request.IgnoreMissingLocalTags {
		return fm_proto.Machine{},
			errors.New("this is a read-only Fleet Manager: full machine information is not available")
	}
	hypervisor, err := m.getLockedHypervisor(request.Hostname, false)
	if err != nil {
		return fm_proto.Machine{}, err
	} else {
		defer hypervisor.mutex.RUnlock()
		return *hypervisor.getMachineLocked(), nil
	}
}

func (m *Manager) getTopology() (*topology.Topology, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.topology == nil {
		return nil, errors.New("no topology available")
	}
	return m.topology, nil
}
