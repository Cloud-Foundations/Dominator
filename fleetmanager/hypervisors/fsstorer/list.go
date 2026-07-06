package fsstorer

import (
	"net"
)

func (s *Storer) listHypervisors() ([]net.IP, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	ips := make([]net.IP, 0, len(s.hypervisorToIPs))
	for ip := range s.hypervisorToIPs {
		ips = append(ips, net.IP(ip[:]))
	}
	return ips, nil
}
