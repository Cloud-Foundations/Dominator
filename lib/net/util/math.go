package util

import (
	"errors"
	"net"
)

var invertTable [256]byte

func init() {
	for value := 0; value < 256; value++ {
		invertTable[value] = invertByte(byte(value))
	}
}

func compareIPs(left, right net.IP) bool {
	leftVal, err := ipToValue(left)
	if err != nil {
		return false
	}
	rightVal, err := ipToValue(right)
	if err != nil {
		return false
	}
	return leftVal < rightVal
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

func ipToValue(ip net.IP) (uint32, error) {
	ip = ip.To4()
	if ip == nil {
		return 0, errors.New("not an IPv4 address")
	}
	return uint32(ip[0])<<24 |
		uint32(ip[1])<<16 |
		uint32(ip[2])<<8 |
		uint32(ip[3]), nil
}
