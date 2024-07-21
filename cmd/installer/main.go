//go:build linux
// +build linux

package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/debuglogger"
	"github.com/Cloud-Foundations/Dominator/lib/logbuf"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

const logfile = "/var/log/installer/latest"

type flusher interface {
	Flush() error
}

type Rebooter interface {
	Reboot() error
	String() string
}

var (
	dryRun = flag.Bool("dryRun", ifUnprivileged(),
		"If true, do not make changes")
	mountPoint = flag.String("mountPoint", "/mnt",
		"Mount point for new root file-system")
	objectsDirectory = flag.String("objectsDirectory", "/objects",
		"Directory where cached objects will be written")
	logDebugLevel = flag.Int("logDebugLevel", -1, "Debug log level")
	portNum       = flag.Uint("portNum", constants.InstallerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	procDirectory = flag.String("procDirectory", "/proc",
		"Directory where procfs is mounted")
	skipNetwork = flag.Bool("skipNetwork", false,
		"If true, do not update target network configuration")
	skipStorage = flag.Bool("skipStorage", false,
		"If true, do not update storage")
	sysfsDirectory = flag.String("sysfsDirectory", "/sys",
		"Directory where sysfs is mounted")
	tftpDirectory = flag.String("tftpDirectory", "/tftpdata",
		"Directory containing (possibly injected) TFTP data")
	tmpRoot = flag.String("tmpRoot", "/tmproot",
		"Mount point for temporary (tmpfs) root file-system")
)

func copyLogs(logFlusher flusher) error {
	logFlusher.Flush()
	logdir := filepath.Join(*mountPoint, "var", "log", "installer")
	return fsutil.CopyFile(filepath.Join(logdir, "log"), logfile,
		fsutil.PublicFilePerms)
}

func createLogger() (*logbuf.LogBuffer, log.DebugLogger) {
	os.MkdirAll("/var/log/installer", fsutil.DirPerms)
	options := logbuf.GetStandardOptions()
	options.AlsoLogToStderr = true
	logBuffer := logbuf.NewWithOptions(options)
	logger := debuglogger.New(stdlog.New(logBuffer, "", 0))
	logger.SetLevel(int16(*logDebugLevel))
	srpc.SetDefaultLogger(logger)
	return logBuffer, logger
}

func ifUnprivileged() bool {
	if os.Geteuid() != 0 {
		return true
	}
	return false
}

func install(updateHwClock bool, logFlusher flusher,
	logger log.DebugLogger) (Rebooter, error) {
	var rebooter Rebooter
	machineInfo, interfaces, err := configureLocalNetwork(logger)
	if err != nil {
		return nil, err
	}
	if !*skipStorage {
		rebooter, err = configureStorage(*machineInfo, logger)
		if err != nil {
			return nil, err
		}
		if !*dryRun && updateHwClock {
			if err := run("hwclock", *tmpRoot, logger, "-w"); err != nil {
				logger.Printf("Error updating hardware clock: %s\n", err)
			} else {
				logger.Println("Updated hardware clock from system clock")
			}
		}
	}
	if !*skipNetwork {
		err := configureNetwork(*machineInfo, interfaces, logger)
		if err != nil {
			return nil, err
		}
	}
	if err := copyLogs(logFlusher); err != nil {
		return nil, fmt.Errorf("error copying logs: %s", err)
	}
	if err := unmountStorage(logger); err != nil {
		return nil, fmt.Errorf("error unmounting: %s", err)
	}
	return rebooter, nil
}

func printAndWait(initialTimeoutString, waitTimeoutString string,
	waitGroup *sync.WaitGroup, rebooterName string, logger log.Logger) {
	initialTimeout, _ := time.ParseDuration(initialTimeoutString)
	if initialTimeout < time.Second {
		initialTimeout = time.Second
		initialTimeoutString = "1s"
	}
	logger.Printf("waiting %s before rebooting with %s rebooter\n",
		initialTimeoutString, rebooterName)
	time.Sleep(initialTimeout - time.Second)
	waitChannel := make(chan struct{})
	go func() {
		waitGroup.Wait()
		waitChannel <- struct{}{}
	}()
	timer := time.NewTimer(time.Second)
	select {
	case <-timer.C:
	case <-waitChannel:
		return
	}
	logger.Printf(
		"waiting %s for remote shells to terminate before rebooting\n",
		waitTimeoutString)
	waitTimeout, _ := time.ParseDuration(waitTimeoutString)
	timer.Reset(waitTimeout)
	select {
	case <-timer.C:
	case <-waitChannel:
	}
}

func doMain() error {
	if err := loadflags.LoadForDaemon("installer"); err != nil {
		return err
	}
	flag.Parse()
	tricorder.RegisterFlags()
	logBuffer, logger := createLogger()
	defer logBuffer.Flush()
	var updateHwClock bool
	if fi, err := os.Stat("/build-timestamp"); err != nil {
		return err
	} else {
		now := time.Now()
		if fi.ModTime().After(now) {
			timeval := syscall.Timeval{Sec: fi.ModTime().Unix()}
			if err := syscall.Settimeofday(&timeval); err != nil {
				return err
			}
			logger.Printf("System time: %s is earlier than build time: %s.\nAdvancing to build time",
				now, fi.ModTime())
			updateHwClock = true
		}
	}
	go runShellOnConsole(logger)
	AddHtmlWriter(logBuffer)
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		logger.Println(err)
	}
	waitGroup := &sync.WaitGroup{}
	if newLogger, err := startServer(*portNum, waitGroup, logger); err != nil {
		logger.Printf("cannot start server: %s\n", err)
	} else {
		logger = newLogger
	}
	rebooter, err := install(updateHwClock, logBuffer, logger)
	rebooterName := "default"
	if rebooter != nil {
		rebooterName = rebooter.String()
	}
	if err != nil {
		logger.Println(err)
		printAndWait("5m", "1h", waitGroup, rebooterName, logger)
	} else {
		printAndWait("5s", "5m", waitGroup, rebooterName, logger)
	}
	syscall.Sync()
	if rebooter != nil {
		if err := rebooter.Reboot(); err != nil {
			logger.Printf("error calling %s rebooter: %s\n", rebooterName, err)
			logger.Println("falling back to default rebooter after 5 minutes")
			time.Sleep(time.Minute * 5)
		}
	}
	if err := syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART); err != nil {
		logger.Fatalf("error rebooting: %s\n", err)
	}
	return nil
}

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
