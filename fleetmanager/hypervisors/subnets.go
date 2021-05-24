package hypervisors

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
)

// This must be called with the lock held.
func (m *Manager) checkIpReserved(tSubnet *topology.Subnet, ip net.IP) bool {
	if ip.Equal(tSubnet.IpGateway) {
		return true
	}
	ipAddr := ip.String()
	if tSubnet.CheckIfIpIsReserved(ipAddr) {
		return true
	}
	if _, ok := m.allocatingIPs[ipAddr]; ok {
		return true
	}
	if _, ok := m.migratingIPs[ipAddr]; ok {
		return true
	}
	return false
}

// This must be called with the lock held. This will update the allocatingIPs
// map.
func (m *Manager) findFreeIPs(tSubnet *topology.Subnet,
	numNeeded uint) ([]net.IP, error) {
	var freeIPs []net.IP
	gatewayIp := tSubnet.IpGateway.String()
	subnet, ok := m.subnets[gatewayIp]
	if !ok {
		return nil, fmt.Errorf("subnet for gateway: %s not found", gatewayIp)
	}
	initialIp := util.CopyIP(subnet.nextIp)
	for numNeeded > 0 {
		if !m.checkIpReserved(subnet.subnet, subnet.nextIp) {
			registered, err := m.storer.CheckIpIsRegistered(subnet.nextIp)
			if err != nil {
				return nil, err
			}
			if !registered {
				freeIPs = append(freeIPs, util.CopyIP(subnet.nextIp))
				numNeeded--
			}
		}
		util.IncrementIP(subnet.nextIp)
		if subnet.nextIp.Equal(subnet.stopIp) {
			copy(subnet.nextIp, subnet.startIp)
		}
		if initialIp.Equal(subnet.nextIp) {
			break
		}
	}
	for _, ip := range freeIPs {
		m.allocatingIPs[ip.String()] = struct{}{}
	}
	return freeIPs, nil
}

func (m *Manager) makeSubnet(tSubnet *topology.Subnet) *subnetType {
	networkIp := tSubnet.IpGateway.Mask(net.IPMask(tSubnet.IpMask))
	var startIp, stopIp net.IP
	if len(tSubnet.FirstAutoIP) > 0 {
		startIp = tSubnet.FirstAutoIP
	} else {
		startIp = util.CopyIP(networkIp)
		util.IncrementIP(startIp)
	}
	if len(tSubnet.LastAutoIP) > 0 {
		stopIp = util.CopyIP(tSubnet.LastAutoIP)
		util.IncrementIP(stopIp)
	} else {
		stopIp = make(net.IP, len(networkIp))
		invertedMask := util.CopyIP(tSubnet.IpMask)
		util.InvertIP(invertedMask)
		for index, value := range invertedMask {
			stopIp[index] = networkIp[index] | value
		}
	}
	return &subnetType{
		subnet:  tSubnet,
		startIp: startIp,
		stopIp:  stopIp,
		nextIp:  util.CopyIP(startIp),
	}
}

func (m *Manager) unmarkAllocatingIPs(ips []net.IP) {
	if len(ips) < 1 {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, ip := range ips {
		delete(m.allocatingIPs, ip.String())
	}
}
