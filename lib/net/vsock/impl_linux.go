package vsock

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	vsockDev = "/dev/vsock"
)

func getContextID() (uint32, error) {
	fd, err := unix.Open(vsockDev, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("error opening %s: %s", vsockDev, err)
	}
	defer unix.Close(fd)
	return unix.IoctlGetUint32(fd,
		unix.IOCTL_VM_SOCKETS_GET_LOCAL_CID)
}
