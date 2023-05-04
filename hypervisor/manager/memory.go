package manager

import (
	"errors"
	"os"
	"os/exec"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/meminfo"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var (
	errorInsufficientAvailableMemory = errors.New(
		"insufficient available memory")
	errorInsufficientUnallocatedMemory = errors.New(
		"insufficient unallocated memory")
	errorUnableToAllocatedMemory = errors.New("unable to allocate memory")
)

func checkAvailableMemory(memoryInMiB uint64) error {
	if memInfo, err := meminfo.GetMemInfo(); err != nil {
		return err
	} else {
		if memoryInMiB >= memInfo.Available>>20 {
			return errorInsufficientAvailableMemory
		}
		return nil
	}
}

func getVmInfoMemoryInMiB(vmInfo proto.VmInfo) uint64 {
	var memoryTotal uint64
	for _, volume := range vmInfo.Volumes {
		if volume.Type == proto.VolumeTypeMemory {
			memoryTotal += volume.Size
		}
	}
	memoryInMiB := memoryTotal >> 20
	if memoryInMiB<<20 < memoryTotal {
		memoryInMiB += 1
	}
	return vmInfo.MemoryInMiB + memoryInMiB
}

func tryAllocateMemory(memoryInMiB uint64) <-chan error {
	channel := make(chan error, 1)
	executable, err := os.Executable()
	if err != nil {
		channel <- err
		return channel
	}
	cmd := exec.Command(executable, "-testMemoryAvailable",
		strconv.FormatUint(memoryInMiB, 10))
	go func() {
		if err := cmd.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				channel <- errorUnableToAllocatedMemory
			} else {
				channel <- err
			}
		} else {
			channel <- nil
		}
	}()
	return channel
}

// This will grab the Manager lock and the lock for each VM.
func (m *Manager) getUnallocatedMemoryInMiB() uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.getUnallocatedMemoryInMiBWithLock(nil)
}

// This will grab the lock for each VM, except a specified VM which should
// already be locked.
func (m *Manager) getUnallocatedMemoryInMiBWithLock(locked *vmInfoType) uint64 {
	unallocated := int64(m.memTotalInMiB)
	for _, vm := range m.vms {
		unallocated -= int64(vm.getMemoryInMiB(vm != locked))
	}
	if unallocated < 0 {
		return 0
	}
	return uint64(unallocated)
}

// This will grab the lock for each VM, except a specified VM which should
// already be locked.
func (m *Manager) checkSufficientMemoryWithLock(memoryInMiB uint64,
	locked *vmInfoType) error {
	if memoryInMiB > m.getUnallocatedMemoryInMiBWithLock(locked) {
		return errorInsufficientUnallocatedMemory
	}
	return checkAvailableMemory(memoryInMiB)
}

func (vm *vmInfoType) getMemoryInMiB(grabLock bool) uint64 {
	if grabLock {
		vm.mutex.RLock()
		defer vm.mutex.RUnlock()
	}
	return getVmInfoMemoryInMiB(vm.VmInfo)
}
