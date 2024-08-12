//go:build linux
// +build linux

package main

import (
	"bufio"
	"bytes"
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
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	"github.com/d2g/dhcp4"
	dhcp "github.com/krolaw/dhcp4" // Used for option strings.
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

func formatText(data []byte) string {
	for _, ch := range data {
		if ch < 0x20 || ch > 0x7e {
			return ""
		}
	}
	return "(\"" + string(data) + "\")"
}

// getVariableFromBytes will search for a "name=value" tuple in a
// space separated slice of bytes. It will return the value if found.
func getVariableFromBytes(data []byte, name string) []byte {
	equals := []byte("=")
	nameBytes := []byte(name)
	for _, arg := range bytes.Fields(data) {
		if fields := bytes.Split(arg, equals); len(fields) == 2 {
			if bytes.Equal(fields[0], nameBytes) {
				return fields[1]
			}
		}
	}
	return nil
}

// isValidHostname returns true if the specified hostname contains valid
// characters.
func isValidHostname(hostname []byte) bool {
	for _, ch := range hostname {
		if (ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') ||
			(ch == '-') {
			continue
		}
		return false
	}
	return true
}

func logDhcpPacket(ifName string, packet dhcp4.Packet,
	options dhcp4.Options) (string, error) {
	topdir := filepath.Join("/var", "log", "installer", "dhcp")
	if err := os.MkdirAll(topdir, fsutil.DirPerms); err != nil {
		return "", err
	}
	// Brute-force way to create the next log directory.
	var logdir string
	for count := 0; true; count++ {
		if count > 100 {
			return "",
				fmt.Errorf("reached DHCP logging limit: empty out: %s", topdir)
		}
		logdir = fmt.Sprintf("%s/%d", topdir, count)
		if err := os.Mkdir(logdir, fsutil.DirPerms); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		break
	}
	err := os.WriteFile(filepath.Join(logdir, "interface"), []byte(ifName),
		fsutil.PublicFilePerms)
	if err != nil {
		return "", err
	}
	if file, err := os.Create(filepath.Join(logdir, "packet")); err != nil {
		return "", err
	} else {
		file.Write(packet)
		file.Close()
	}
	optionsFile, err := os.Create(filepath.Join(logdir, "options"))
	if err != nil {
		return "", err
	}
	defer optionsFile.Close()
	writer := bufio.NewWriter(optionsFile)
	defer writer.Flush()
	for code, value := range options {
		stringCode := dhcp.OptionCode(code).String()
		fmt.Fprintf(writer, "Code: %3d/%s\n", code, stringCode)
		fmt.Fprintf(writer, "  value: %#x%s\n", value, formatText(value))
		optionFilename := filepath.Join(logdir,
			fmt.Sprintf("option.%d_%s", code, stringCode))
		if file, err := os.Create(optionFilename); err != nil {
			return "", err
		} else {
			file.Write(value)
			file.Close()
		}
	}
	return logdir, nil
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

// readHostnameFromKernelCmdline will read the kernel command-line and will
// return the value of the "hostname=" argument if found.
func readHostnameFromKernelCmdline() []byte {
	if data, err := readKernelCmdline(); err == nil {
		return getVariableFromBytes(data, "hostname")
	}
	return nil
}

func readKernelCmdline() ([]byte, error) {
	return os.ReadFile(filepath.Join(*procDirectory, "cmdline"))
}

// readMbr will read the MBR from a file. It returns an error if there is a
// problem opening or reading the file. If there is no MBR signature, a nil
// object is returned along with no error.
func readMbr(filename string) (*mbr.Mbr, error) {
	if file, err := os.Open(filename); err != nil {
		return nil, err
	} else {
		defer file.Close()
		return mbr.Decode(file)
	}
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
