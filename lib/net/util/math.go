package util

import (
	"net"
)

var invertTable [256]byte

func init() {
	for value := 0; value < 256; value++ {
		invertTable[value] = invertByte(byte(value))
	}
}

func copyIP(ip net.IP) net.IP {
	retval := make(net.IP, len(ip))
	copy(retval, ip)
	return retval
}

func decrementIP(ip net.IP) {
	for index := len(ip) - 1; index >= 0; index-- {
		if ip[index] > 0 {
			ip[index]--
			return
		}
		ip[index] = 0xff
	}
}

func incrementIP(ip net.IP) {
	for index := len(ip) - 1; index >= 0; index-- {
		if ip[index] < 255 {
			ip[index]++
			return
		}
		ip[index] = 0
	}
}

func invertByte(input byte) byte {
	var inverted byte
	for index := 0; index < 8; index++ {
		inverted <<= 1
		if input&0x80 == 0 {
			inverted |= 1
		}
		input <<= 1
	}
	return inverted
}

func invertIP(ip net.IP) {
	for index, value := range ip {
		ip[index] = invertTable[value]
	}
}
