// +build !linux

package vsock

import "syscall"

func getContextID() (uint32, error) {
	return 0, syscall.ENOTSUP
}
