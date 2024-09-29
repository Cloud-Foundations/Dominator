package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func runShellOnConsole(logger log.DebugLogger) {
	for {
		logger.Println("starting shell on console")
		cmd := exec.Command(shellCommand[0], shellCommand[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logger.Printf("error running shell: %s\n", err)
			if os.IsNotExist(err) {
				break
			}
		}
		if *dryRun {
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}
	}
}
