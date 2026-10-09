package rpcd

import (
	"runtime"
	"syscall"
)

func lowerPriority() {
	runtime.LockOSThread()
	syscall.Setpriority(syscall.PRIO_PROCESS, 0, 10)
}
