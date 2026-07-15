package allocator

import (
	"errors"
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type hypervisorAllocation struct {
	memoryInMiB      uint64
	milliCPUs        uint64
	subnets          map[string]uint // Key: subnet ID, value: num IPs used.
	totalVolumeBytes types.Bytes     // TODO(rgooch): split by backing store.
}

var (
	errorCannotFit = errors.New("cannot fit")
	errorListError = errors.New("list error")
)

func addVmAllocationToTotals(vmSpec fm_proto.VmAllocationSpecification,
	hypervisorHostname string, totals map[string]*hypervisorAllocation) {
	hAlloc := totals[hypervisorHostname]
	if hAlloc == nil {
		hAlloc = &hypervisorAllocation{
			subnets: make(map[string]uint),
		}
		totals[hypervisorHostname] = hAlloc
	}
	hAlloc.memoryInMiB += vmSpec.MemoryInMiB
	hAlloc.milliCPUs += uint64(vmSpec.MilliCPUs)
	for _, netif := range vmSpec.NetworkInterfaces {
		hAlloc.subnets[netif.SubnetId]++
	}
	for _, volume := range vmSpec.Volumes {
		hAlloc.totalVolumeBytes += volume.Size
	}
}

// checkVmFitsOnMachine returns true if the VM will fit on the machine at this
// time. The errorCannotFit error is returned if the VM will never fit.
func (m *manager) checkVmFitsOnMachine(vm fm_proto.VmAllocationSpecification,
	machine *fm_proto.Machine,
	hypervisorAllocations map[string]*hypervisorAllocation) (bool, error) {
	if vm.HypervisorArchitecture != hyper_proto.ArchitectureTypeAuto &&
		vm.HypervisorArchitecture != machine.ArchitectureType {
		return false, errorCannotFit
	}
	var totalVolumeSize types.Bytes
	for _, volume := range vm.Volumes {
		totalVolumeSize += volume.Size
	}
	// Fast initial check to see if VM could ever fit.
	if vm.MemoryInMiB >= machine.MemoryInMiB ||
		uint64(vm.MilliCPUs) >= uint64(machine.NumCPUs)*1000 ||
		totalVolumeSize >= types.Bytes(machine.TotalVolumeBytes) {
		return false, errorCannotFit
	}
	// TODO(rgooch): avoid recomputing this every time.
	subnetsList, err := m.topology.GetSubnetsForMachine(machine.Hostname)
	if err != nil {
		return false, err
	}
	subnetsMap := make(map[string]struct{}, len(subnetsList))
	for _, subnet := range subnetsList {
		subnetsMap[subnet.Id] = struct{}{}
	}
	for _, netif := range vm.NetworkInterfaces {
		if _, exists := subnetsMap[netif.SubnetId]; !exists {
			return false, errorCannotFit
		}
	}
	hyperData, ok := m.hypervisorDatas[machine.Hostname]
	if !ok {
		return false, nil
	}
	hAlloc := hypervisorAllocations[machine.Hostname]
	if hAlloc == nil {
		hAlloc = &hypervisorAllocation{
			subnets: make(map[string]uint),
		}
		hypervisorAllocations[machine.Hostname] = hAlloc
	}
	if vm.MemoryInMiB+hAlloc.memoryInMiB+hyperData.AllocatedMemory >=
		machine.MemoryInMiB {
		return false, nil
	}
	if uint64(vm.MilliCPUs)+hAlloc.milliCPUs+hyperData.AllocatedMilliCPUs >=
		uint64(machine.NumCPUs)*1000 {
		return false, nil
	}
	for _, netif := range vm.NetworkInterfaces {
		numFree := hyperData.NumFreeAddresses[netif.SubnetId]
		if hAlloc.subnets[netif.SubnetId] >= numFree {
			return false, nil
		}
	}
	if hyperData.AvailableMemory > 0 &&
		vm.MemoryInMiB >= hyperData.AvailableMemory {
		return false, nil
	}
	if totalVolumeSize+hAlloc.totalVolumeBytes+
		types.Bytes(hyperData.AllocatedVolumeBytes) >=
		types.Bytes(machine.TotalVolumeBytes) {
		return false, nil
	}
	return true, nil
}

func (m *manager) computeAllocationTotals() map[string]*hypervisorAllocation {
	totals := make(map[string]*hypervisorAllocation)
	for _, allocation := range m.allocations {
		for vmIndex, vmSpec := range allocation.request.VMs {
			if _, ok := allocation.indexToVmIp[vmIndex]; ok {
				continue // Appears in summaries.
			}
			hypervisorHostname := allocation.vmHypervisors[vmIndex]
			addVmAllocationToTotals(vmSpec, hypervisorHostname, totals)
		}
	}
	return totals
}

// listMachinesInLocation will list the machines in a location.
func (m *manager) listMachinesInLocation(location string) (
	[]*fm_proto.Machine, error) {
	topoMachines, err := m.topology.ListMachines(location)
	if err != nil {
		return nil, err
	}
	machines := make([]*fm_proto.Machine, 0, len(topoMachines))
	for _, tm := range topoMachines {
		if machine, ok := m.machines[tm.Hostname]; !ok {
			return nil, fmt.Errorf("unknown machine: %s", tm.Hostname)
		} else {
			machines = append(machines, machine)
		}
	}
	return machines, nil
}

// recalculate will process the request queue and make an allocation if there is
// capacity available to satisfy a request. It returns the RequestId for the
// allocation made, else "".
func (m *manager) recalculate() fm_proto.RequestId {
	hypervisorAllocations := m.computeAllocationTotals()
	var allocationRequestId fm_proto.RequestId
	m.walkQueue(func(request fm_proto.AllocateRequestEntry) bool {
		allocation, vmHypervisors, err := m.tryToAllocate(
			request.Request, hypervisorAllocations)
		if err != nil {
			if err := m.dequeue(nil, request.RequestId); err != nil {
				m.params.Logger.Println(err)
				return false
			}
			var deletedEntry fm_proto.DeletedAllocation
			if err == errorCannotFit {
				deletedEntry = fm_proto.DeletedAllocation{
					Reason: fm_proto.AllocationRequestCannotFit,
				}
			} else {
				deletedEntry = fm_proto.DeletedAllocation{Error: err.Error()}
			}
			m.deleted[request.RequestId] = &deletedType{
				deleted:  &deletedEntry,
				request:  &request.Request,
				username: request.Username,
			}
			err = m.sendDeleted(request.RequestId, &request.Request,
				deletedEntry, request.Username)
			if err != nil {
				m.params.Logger.Println(err)
			}
			return false
		}
		if allocation == nil {
			return false
		}
		if err := m.dequeue(nil, request.RequestId); err != nil {
			m.params.Logger.Println(err)
			return false
		}
		err = m.sendAllocation(request.RequestId, &request.Request, allocation,
			request.Username)
		if err != nil {
			m.params.Logger.Println(err)
			return false
		}
		m.allocations[request.RequestId] = &allocationType{
			allocation:    allocation,
			request:       &request.Request,
			vmHypervisors: vmHypervisors,
			username:      request.Username,
		}
		delete(m.waitingRequestsById, request.RequestId)
		return false
	})
	return allocationRequestId
}

// tryToAllocate will try to allocate capacity for the request. It returns the
// allocation made and the hypervisor hostnames, else nil.
// The errorCannotFit error is returned if any of the VMs will never fit.
func (m *manager) tryToAllocate(req fm_proto.AllocateRequest,
	hypervisorAllocations map[string]*hypervisorAllocation) (
	*fm_proto.Allocation, []string, error) {
	var hypervisorHostnames []string
	vmAllocations := make([]fm_proto.VmAllocation, 0, len(req.VMs))
	for _, vm := range req.VMs {
		vmAllocation, hypervisorHostname, err := m.tryToAllocateVm(vm,
			hypervisorAllocations)
		if err != nil {
			return nil, nil, err
		}
		if vmAllocation == nil {
			return nil, nil, nil
		}
		hypervisorHostnames = append(hypervisorHostnames, hypervisorHostname)
		vmAllocations = append(vmAllocations, *vmAllocation)
	}
	if len(vmAllocations) < 1 {
		return nil, nil, nil
	}
	return &fm_proto.Allocation{
		CreateDeadline: time.Now().Add(m.options.CreateDeadline),
		VMs:            vmAllocations,
	}, hypervisorHostnames, nil
}

// tryToAllocateVm will try to allocate capacity for a VM. It returns the
// allocation made and the hypervisor hostname, else nil.
// The errorCannotFit error is returned if the VM will never fit.
func (m *manager) tryToAllocateVm(vm fm_proto.VmAllocationSpecification,
	hypervisorAllocations map[string]*hypervisorAllocation) (
	*fm_proto.VmAllocation, string, error) {
	machines, err := m.listMachinesInLocation(vm.Location)
	if err != nil {
		return nil, "", err
	}
	tagsToMatch := tagmatcher.New(vm.HypervisorTagsToMatch, false)
	var matchedSome bool
	for _, machine := range machines {
		if !tagsToMatch.MatchEach(machine.Tags) {
			continue
		}
		fits, err := m.checkVmFitsOnMachine(vm, machine, hypervisorAllocations)
		if err == errorCannotFit {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		matchedSome = true
		if !fits {
			continue
		}
		addVmAllocationToTotals(vm, machine.Hostname, hypervisorAllocations)
		address := fmt.Sprintf("%s:%d",
			machine.Hostname, constants.HypervisorPortNumber)
		return &fm_proto.VmAllocation{HypervisorAddress: address},
			machine.Hostname,
			nil
	}
	if !matchedSome {
		return nil, "", errorCannotFit
	}
	return nil, "", nil
}
