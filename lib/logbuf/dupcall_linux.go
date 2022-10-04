package logbuf

import (
	"syscall"
)

// Arm64 linux does NOT support the Dup2 syscall
// https://marcin.juszkiewicz.com.pl/download/tables/syscalls.html
// and dup3 is more supported so doing it here:
func localDup(oldfd int, newfd int) error {
	return syscall.Dup3(oldfd, newfd, 0)
}
