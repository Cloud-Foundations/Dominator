package logbuf

import (
	"syscall"
)

func localDup(oldfd int, newfd int) error {
	return syscall.Dup2(oldfd, newfd)
}
