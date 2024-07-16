package osutil

import (
	"errors"
	"os/exec"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type flusher interface {
	Flush() error
}

func hardReboot(logger log.Logger) error {
	syncAndWait(logger)
	syncAndWait(logger)
	syncAndWait(logger)
	logger.Println("Calling reboot() system call and wait")
	if logger, ok := logger.(flusher); ok {
		logger.Flush()
	}
	time.Sleep(time.Second)
	errorChannel := make(chan error, 1)
	timer := time.NewTimer(time.Second)
	go func() {
		errorChannel <- wsyscall.Reboot()
	}()
	select {
	case <-timer.C:
		return errors.New("still alive after a hard reboot. Waaah!")
	case err := <-errorChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return err
	}
}

// Returns true on success, else false.
func runCommand(logger log.Logger, name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	if logs, err := cmd.CombinedOutput(); err != nil {
		errMsg := "error running: " + name
		for _, arg := range args {
			errMsg += " " + arg
		}
		errMsg += ": " + err.Error()
		logger.Println(errMsg)
		logger.Println(string(logs))
		return false
	}
	return true
}

// runCommandBackground returns a channel that receives a message if the command
// fails.
func runCommandBackground(logger log.Logger, name string,
	args ...string) <-chan struct{} {
	failureChannel := make(chan struct{}, 1)
	go func() {
		if !RunCommand(logger, name, args...) {
			failureChannel <- struct{}{}
		}
	}()
	return failureChannel
}

// syncAndWait will try to sync file-system data and then waits up to 5 seconds
// for it to complete.
func syncAndWait(logger log.Logger) {
	logger.Println("Calling sync() system call and wait")
	timer := time.NewTimer(5 * time.Second)
	waitChannel := make(chan struct{}, 1)
	go func() {
		wsyscall.Sync()
		waitChannel <- struct{}{}
	}()
	select {
	case <-timer.C:
		logger.Println("Timed out waiting for sync() system call")
		return
	case <-waitChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return
	}
}
