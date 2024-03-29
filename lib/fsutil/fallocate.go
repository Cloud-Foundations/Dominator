package fsutil

import (
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func fallocate(filename string, size uint64) error {
	fd, err := syscall.Open(filename, syscall.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	return wsyscall.Fallocate(int(fd), wsyscall.FALLOC_FL_KEEP_SIZE,
		0, int64(size))
}
