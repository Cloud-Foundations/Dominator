package manager

import (
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (m *Manager) closeUpdateChannel(channel <-chan proto.Update) {
	m.notifiersMutex.Lock()
	defer m.notifiersMutex.Unlock()
	delete(m.notifiers, channel)
}

func (m *Manager) getHealthStatus() string {
	m.healthStatusMutex.RLock()
	defer m.healthStatusMutex.RUnlock()
	return m.healthStatus
}

func (m *Manager) makeUpdateChannel() <-chan proto.Update {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	subnets := make([]proto.Subnet, 0, len(m.subnets))
	for id, subnet := range m.subnets {
		if id != "hypervisor" {
			subnets = append(subnets, subnet)
		}
	}
	vms := make(map[string]*proto.VmInfo, len(m.vms))
	for addr, vm := range m.vms {
		vms[addr] = &vm.VmInfo
	}
	numFreeAddresses, err := m.computeNumFreeAddressesMap(m.addressPool)
	if err != nil {
		m.Logger.Println(err)
	}
	channel := make(chan proto.Update, 16)
	m.notifiersMutex.Lock()
	defer m.notifiersMutex.Unlock()
	m.notifiers[channel] = channel
	// Initial update: give everything.
	channel <- proto.Update{
		HaveAddressPool:  true,
		AddressPool:      m.addressPool.Registered,
		HaveDisabled:     true,
		Disabled:         m.disabled,
		MemoryInMiB:      &m.memTotalInMiB,
		NumCPUs:          &m.numCPUs,
		NumFreeAddresses: numFreeAddresses,
		HealthStatus:     m.healthStatus,
		HaveSerialNumber: true,
		SerialNumber:     m.serialNumber,
		HaveSubnets:      true,
		Subnets:          subnets,
		TotalVolumeBytes: &m.totalVolumeBytes,
		HaveVMs:          true,
		VMs:              vms,
	}
	return channel
}

func (m *Manager) sendUpdate(update proto.Update) {
	update.HealthStatus = m.getHealthStatus()
	m.notifiersMutex.Lock()
	defer m.notifiersMutex.Unlock()
	for readChannel, writeChannel := range m.notifiers {
		select {
		case writeChannel <- update:
		default:
			close(writeChannel)
			delete(m.notifiers, readChannel)
		}
	}
}
