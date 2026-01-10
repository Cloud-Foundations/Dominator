package main

import (
	"errors"
	"net"
	"os"
	"strings"
)

type dialerType struct{}

func (d *dialerType) Dial(network, address string) (net.Conn, error) {
	if !*allocateRootPort {
		return net.Dial(network, address)
	}
	if os.Geteuid() != 0 {
		return nil, errors.New("no permission to allocate privileged port")
	}
	for port := 1; port < 1024; port++ {
		d := &net.Dialer{LocalAddr: &net.TCPAddr{Port: port}}
		if conn, err := d.Dial(network, address); err != nil {
			if !strings.Contains(err.Error(), "address already in use") {
				return nil, err
			}
		} else {
			return conn, nil
		}
	}
	return nil, errors.New("no unused privileged ports available")
}
