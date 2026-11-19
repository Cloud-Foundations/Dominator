package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func runWithNewProcSubcommand(args []string, logger log.DebugLogger) error {
	chroot := args[0]
	prog := args[1]
	pargs := args[2:]
	procMount := filepath.Join(chroot, "proc")
	if err := wsyscall.Mount("none", procMount, "proc", 0, ""); err != nil {
		return fmt.Errorf("error mounting /proc: %s", err)
	}
	cmd := exec.Command(prog, pargs...)
	cmd.Dir = "/"
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:       chroot,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	return cmd.Run()
}
