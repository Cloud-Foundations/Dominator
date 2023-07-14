package hypervisors

import (
	"errors"
	"net"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (h *hypervisorType) makeProtoHypervisor(
	includeVMs bool) fm_proto.Hypervisor {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	protoHypervisor := fm_proto.Hypervisor{
		Machine: *h.machine,
	}
	protoHypervisor.AllocatedMilliCPUs = h.allocatedMilliCPUs
	protoHypervisor.AllocatedMemory = h.allocatedMemory
	protoHypervisor.AllocatedVolumeBytes = h.allocatedVolumeBytes
	protoHypervisor.Machine.MemoryInMiB = h.memoryInMiB
	protoHypervisor.NumCPUs = h.numCPUs
	protoHypervisor.TotalVolumeBytes = h.totalVolumeBytes
	if includeVMs {
		protoHypervisor.VMs = make([]hyper_proto.VmInfo, 0, len(h.vms))
		for _, vm := range h.vms {
			protoHypervisor.VMs = append(protoHypervisor.VMs, vm.VmInfo)
		}
	}
	return protoHypervisor
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
		return vm.hypervisor.machine.Hostname, nil
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
