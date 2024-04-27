//go:build !linux

package vsock

import "syscall"

func checkVsockets() error {
	return syscall.ENOTSUP
}

func getContextID() (uint32, error) {
	return 0, syscall.ENOTSUP
}
