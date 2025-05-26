//go:build linux
// +build linux

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/commands"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/format"
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

type logWriter struct {
	writer io.Writer
}

type Rebooter interface {
	Reboot() error
	String() string
}

var (
	completionNotifier = flag.String("completionNotifier", "",
		"Pathname of programme to run when installation is complete and reboot is imminent")
	configurationLoader = flag.String("configurationLoader", "",
		"Pathname of programme to run to load configuration data")
	driveSelector = flag.String("driveSelector", "",
		"Pathname of programme to select drives to configure")
	dryRun = flag.Bool("dryRun", ifUnprivileged(),
		"If true, do not make changes")
	logDebugLevel = flag.Int("logDebugLevel", -1, "Debug log level")
	mountPoint    = flag.String("mountPoint", "/mnt",
		"Mount point for new root file-system")
	networkConfigurator = flag.String("networkConfigurator", "",
		"Pathname of programme to run to configure the network")
	portNum = flag.Uint("portNum", constants.InstallerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	procDirectory = flag.String("procDirectory", "/proc",
		"Directory where procfs is mounted")
	shellCommand = flagutil.StringList{"/bin/busybox", "sh", "-i"}
	skipNetwork  = flag.Bool("skipNetwork", false,
		"If true, do not update target network configuration")
	skipStorage = flag.Bool("skipStorage", false,
		"If true, do not update storage")
	sysfsDirectory = flag.String("sysfsDirectory", "/sys",
		"Directory where sysfs is mounted")
	tftpDirectory = flag.String("tftpDirectory", "/tftpdata",
		"Directory containing (possibly injected) TFTP data")
	tmpRoot = flag.String("tmpRoot", "/tmproot",
		"Mount point for temporary (tmpfs) root file-system")

	processStartTime = time.Now()
)

func init() {
	flag.Var(&shellCommand, "shellCommand",
		"Shell command with optional comma separated arguments")
}

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w,
		"Usage: installer [flags...] [command [args...]]")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"decode-base64", "", 0, 0, decodeBase64Subcommand},
	{"dhcp-request", "", 0, 0, dhcpRequestSubcommand},
	{"generate-random", "", 0, 0, generateRandomSubcommand},
	{"list-images", "", 0, 0, listImagesSubcommand},
	{"load-image", "image-name root-dir", 2, 2, loadImageSubcommand},
}

func copyLogs(logFlusher flusher) error {
	logFlusher.Flush()
	logdir := filepath.Join(*mountPoint, "var", "log", "installer")
	return fsutil.CopyFile(filepath.Join(logdir, "log"), logfile,
		fsutil.PublicFilePerms)
}

func createLogger() (*logbuf.LogBuffer, log.DebugLogger, error) {
	if err := os.MkdirAll("/var/log/installer", fsutil.DirPerms); err != nil {
		return nil, nil, err
	}
	options := logbuf.GetStandardOptions()
	options.AlsoLogToStderr = true
	logBuffer := logbuf.NewWithOptions(options)
	logger := debuglogger.New(stdlog.New(&logWriter{logBuffer}, "", 0))
	logger.SetLevel(int16(*logDebugLevel))
	srpc.SetDefaultLogger(logger)
	return logBuffer, logger, nil
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
	machineInfo, interfaces, activeInterface, err := configureLocalNetwork(
		logger)
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
		err := configureNetwork(*machineInfo, interfaces, activeInterface,
			logger)
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
	if *completionNotifier != "" {
		err := run(*completionNotifier, "", logger, *tftpDirectory,
			activeInterface)
		if err != nil {
			return nil, err
		}
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

func runDaemon() error {
	tricorder.RegisterFlags()
	logBuffer, logger, err := createLogger()
	if err != nil {
		return err
	}
	defer logBuffer.Flush()
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		logger.Printf("Error getting system info: %s\n", err)
	} else {
		logger.Printf("installer started %s after system bootup\n",
			format.Duration(time.Second*time.Duration(sysinfo.Uptime)))
	}
	var updateHwClock bool
	if fi, err := os.Stat("/build-timestamp"); err != nil {
		if !*dryRun {
			return err
		}
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
	if l, e := startServer(*portNum, waitGroup, logBuffer, logger); e != nil {
		logger.Printf("cannot start server: %s\n", e)
	} else {
		logger = l
	}
	rebooter, err := install(updateHwClock, logBuffer, logger)
	if *dryRun {
		if err != nil {
			logger.Printf("error installing: %s\n", err)
		}
		logger.Println("dry run: sleeping indefinitely instead of rebooting")
		select {}
	}
	rebooterName := "default"
	if rebooter != nil {
		rebooterName = rebooter.String()
	}
	if err != nil {
		logger.Printf("error installing: %s\n", err)
		printAndWait("5m", "1h", waitGroup, rebooterName, logger)
	} else {
		printAndWait("5s", "5m", waitGroup, rebooterName, logger)
	}
	logger.Println("installation completed")
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

func processCommand(args []string) {
	if len(args) < 1 {
		if err := runSubcommand(nil, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	logger := debuglogger.New(stdlog.New(os.Stderr, "", 0))
	logger.SetLevel(int16(*logDebugLevel))
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		logger.Println(err)
	}
	os.Exit(commands.RunCommands(subcommands, printUsage, logger))
}

func main() {
	if err := loadflags.LoadForDaemon("installer"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Usage = printUsage
	flag.Parse()
	processCommand(flag.Args())
}

func runSubcommand(args []string, logger log.DebugLogger) error {
	return runDaemon()
}

func (w *logWriter) Write(p []byte) (int, error) {
	buffer := &bytes.Buffer{}
	fmt.Fprintf(buffer, "[%7.3f] ", time.Since(processStartTime).Seconds())
	buffer.Write(p)
	return w.writer.Write(buffer.Bytes())
}
