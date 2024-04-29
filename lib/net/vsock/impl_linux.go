package vsock

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	vsockDev = "/dev/vsock"
)

func checkVsockets() error {
	if fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0); err != nil {
		return err
	} else {
		unix.Close(fd)
		return nil
	}
}

func getContextID() (uint32, error) {
	fd, err := unix.Open(vsockDev, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("error opening %s: %s", vsockDev, err)
	}
	defer unix.Close(fd)
	return unix.IoctlGetUint32(fd,
		unix.IOCTL_VM_SOCKETS_GET_LOCAL_CID)
}
