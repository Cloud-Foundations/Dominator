package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

func runCommandInImageChrootSubcommand(args []string,
	logger log.DebugLogger) error {
	objectsGetter := getObjectsGetter(logger)
	err := runCommandInImageChroot(objectsGetter, args[0], args[1:])
	if err != nil {
		return fmt.Errorf("error making running command in image chroot: %s",
			err)
	}
	return nil
}

func runCommandInImageChroot(objectsGetter objectserver.ObjectsGetter,
	name string, command []string) error {
	if os.Geteuid() != 0 {
		return reExecAsRoot()
	}
	dirname, err := os.MkdirTemp("", "imagetool-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(dirname); err != nil {
			logger.Println(err)
		}
	}()
	err = getImageAndWrite(objectsGetter, name, dirname, logger)
	if err != nil {
		return err
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = "/tmp"
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: dirname,
	}
	return cmd.Run()
}
