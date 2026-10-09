package util

import (
	"net"
)

func splitIPs(ips []net.IP) (ipv4s []net.IP, ipv6s []net.IP) {
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			ipv4s = append(ipv4s, ipv4)
		} else if ipv6 := ip.To16(); ipv6 != nil {
			ipv6s = append(ipv6s, ipv6)
		}
	}
	return
}
