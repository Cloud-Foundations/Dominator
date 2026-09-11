package osutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type flusher interface {
	Flush() error
}

func hardReboot(logger log.Logger) error {
	syncAndWait(5*time.Second, logger)
	syncAndWait(5*time.Second, logger)
	syncAndWait(5*time.Second, logger)
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

func runCommandWithFileOutput(cmd *exec.Cmd,
	stdoutFilename, stderrFilename string, deleteAfter bool) (
	stdoutData, stderrData []byte, err error) {
	stdoutFile, err := os.Create(stdoutFilename)
	if err != nil {
		return nil, nil, err
	}
	stderrFile, err := os.Create(stderrFilename)
	if err != nil {
		stdoutFile.Close()
		return nil, nil, err
	}
	if deleteAfter {
		defer func() {
			os.Remove(stdoutFilename)
			os.Remove(stderrFilename)
		}()
	}
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	err = cmd.Run()
	stdoutFile.Close()
	stderrFile.Close()
	stdoutData, eo := os.ReadFile(stdoutFilename)
	stderrData, ee := os.ReadFile(stderrFilename)
	if err != nil {
		return stdoutData, stderrData, err
	}
	if eo != nil {
		return stdoutData, stderrData, eo
	}
	if ee != nil {
		return stdoutData, stderrData, ee
	}
	return stdoutData, stderrData, nil
}

func syncAndWait(timeout time.Duration, logger log.Logger) {
	logger.Println("Calling sync() system call and wait")
	if err := SyncTimeout(timeout); err != nil {
		logger.Printf("Error syncing: %s\n", err)
	}
}

func syncTimeout(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	waitChannel := make(chan struct{}, 1)
	var err error
	go func() {
		err = wsyscall.Sync()
		waitChannel <- struct{}{}
	}()
	select {
	case <-timer.C:
		return fmt.Errorf("timed out waiting for sync() system call")
	case <-waitChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return err
	}
}
