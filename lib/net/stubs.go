//go:build windows

package net

import (
	"net"
	"syscall"
	"time"
)

func bindAndDial(network, localAddr, remoteAddr string, timeout time.Duration) (
	net.Conn, error) {
	return nil, syscall.ENOTSUP
}

func listenWithReuse(network, address string) (net.Listener, error) {
	return nil, syscall.ENOTSUP
}
