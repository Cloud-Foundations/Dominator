package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) getSummary() *summaryData {
	m.summaryMutex.RLock()
	defer m.summaryMutex.RUnlock()
	return m.summary
}

func (m *Manager) holdLock(timeout time.Duration, writeLock bool) error {
	if timeout > time.Minute {
		return fmt.Errorf("timeout: %s exceeds one minute", timeout)
	}
	if writeLock {
		m.mutex.Lock()
		time.Sleep(timeout)
		m.mutex.Unlock()
	} else {
		m.mutex.RLock()
		time.Sleep(timeout)
		m.mutex.RUnlock()
	}
	return nil
}

func (m *Manager) updateSummaryWithMainRLock() {
	availableMilliCPU := m.getAvailableMilliCPUWithLock()
	memUnallocated := m.getUnallocatedMemoryInMiBWithLock(nil)
	numFreeAddresses := uint(len(m.addressPool.Free))
	numRegisteredAddresses := uint(len(m.addressPool.Registered))
	numRunning, numStopped := m.getNumVMsWithLock()
	numSubnets := uint(len(m.subnets))
	ownerGroups := stringutil.ConvertMapKeysToList(m.ownerGroups, false)
	ownerUsers := stringutil.ConvertMapKeysToList(m.ownerUsers, false)
	summary := &summaryData{
		availableMilliCPU:      availableMilliCPU,
		memUnallocated:         memUnallocated,
		numFreeAddresses:       numFreeAddresses,
		numRegisteredAddresses: numRegisteredAddresses,
		numRunning:             numRunning,
		numStopped:             numStopped,
		numSubnets:             numSubnets,
		ownerGroups:            ownerGroups,
		ownerUsers:             ownerUsers,
		updatedAt:              time.Now(),
	}
	m.summaryMutex.Lock()
	defer m.summaryMutex.Unlock()
	m.summary = summary
}

func (m *Manager) setDisabledState(disable bool) error {
	m.mutex.Lock()
	doUnlock := true
	defer func() {
		if doUnlock {
			m.mutex.Unlock()
		}
	}()
	if m.disabled == disable {
		return nil
	}
	filename := filepath.Join(m.StartOptions.StateDir, "disabled")
	if disable {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			fsutil.PublicFilePerms)
		if err != nil {
			return err
		}
		file.Close()
	} else {
		if err := os.Remove(filename); err != nil {
			return err
		}
	}
	m.disabled = disable
	numFreeAddresses, err := m.computeNumFreeAddressesMap(m.addressPool)
	if err != nil {
		m.Logger.Println(err)
	}
	m.mutex.Unlock()
	doUnlock = false
	m.sendUpdate(proto.Update{
		HaveDisabled:     true,
		Disabled:         disable,
		NumFreeAddresses: numFreeAddresses,
	})
	return nil
}
