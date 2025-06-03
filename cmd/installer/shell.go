package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func runShellOnConsole(logger log.DebugLogger) {
	runtime.LockOSThread()
	for {
		logger.Println("starting shell on console")
		cmd := exec.Command(shellCommand[0], shellCommand[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
		}
		if err := cmd.Run(); err != nil {
			logger.Printf("error running shell: %s\n", err)
			if os.IsNotExist(err) {
				break
			}
			time.Sleep(5 * time.Second)
		}
		if *dryRun {
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}
	}
}
