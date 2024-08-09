//go:build linux
// +build linux

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type writeCloser struct{}

var standardBindMounts = []string{"dev", "proc", "sys", "tmp"}

func create(filename string) (io.WriteCloser, error) {
	if *dryRun {
		return &writeCloser{}, nil
	}
	return os.Create(filename)
}

func findExecutable(rootDir, file string) error {
	if d, err := os.Stat(filepath.Join(rootDir, file)); err != nil {
		return err
	} else {
		if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
			return nil
		}
		return os.ErrPermission
	}
}

func lookPath(rootDir, file string) (string, error) {
	if strings.Contains(file, "/") {
		if err := findExecutable(rootDir, file); err != nil {
			return "", err
		}
		return file, nil
	}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "." // Unix shell semantics: path element "" means "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(rootDir, path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("(chroot=%s) %s not found in PATH", rootDir, file)
}

// readString will read a string from the specified filename.
// If the file does not exist an empty string is returned if ignoreMissing is
// true, else an error is returned.
func readString(filename string, ignoreMissing bool) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if ignoreMissing && os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func run(name, chroot string, logger log.DebugLogger, args ...string) error {
	if *dryRun {
		logger.Debugf(0, "dry run: skipping: %s %s\n",
			name, strings.Join(args, " "))
		return nil
	}
	path, err := lookPath(chroot, name)
	if err != nil {
		return err
	}
	cmd := exec.Command(path, args...)
	cmd.WaitDelay = time.Second
	if chroot != "" {
		cmd.Dir = "/"
		cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: chroot}
		logger.Debugf(0, "running(chroot=%s): %s %s\n",
			chroot, name, strings.Join(args, " "))
	} else {
		logger.Debugf(0, "running: %s %s\n", name, strings.Join(args, " "))
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		if err == exec.ErrWaitDelay {
			return nil
		}
		return fmt.Errorf("error running: %s: %s, output: %s",
			name, err, output)
	} else {
		return nil
	}
}

func unpackAndMount(rootDir string, fileSystem *filesystem.FileSystem,
	objGetter objectserver.ObjectsGetter, doInTmpfs bool,
	logger log.DebugLogger) error {
	if err := os.MkdirAll(rootDir, fsutil.DirPerms); err != nil {
		return err
	}
	for _, mountPoint := range standardBindMounts {
		syscall.Unmount(filepath.Join(rootDir, mountPoint), 0)
	}
	syscall.Unmount(rootDir, 0)
	if doInTmpfs {
		if err := wsyscall.Mount("none", rootDir, "tmpfs", 0, ""); err != nil {
			return err
		}
	}
	if err := util.Unpack(fileSystem, objGetter, rootDir, logger); err != nil {
		return err
	}
	for _, mountPoint := range standardBindMounts {
		err := wsyscall.Mount("/"+mountPoint,
			filepath.Join(rootDir, mountPoint), "",
			wsyscall.MS_BIND, "")
		if err != nil {
			return err
		}
	}
	return nil
}

func (wc *writeCloser) Close() error {
	return nil
}

func (wc *writeCloser) Write(p []byte) (int, error) {
	return len(p), nil
}
