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
	"os/signal"
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
	imageServerHostname = flag.String("imageServerHostname", "",
		"Hostname of image server (overrides TFTP data)")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server (overrides TFTP data)")
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
	tftpServerHostname = flag.String("tftpServerHostname", "",
		"Hostname of TFTP server (overrides DHCP response)")
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
	{"kexec-image", "image-name", 1, 1, kexecImageSubcommand},
	{"list-drives", "", 0, 0, listDrivesSubcommand},
	{"list-images", "", 0, 0, listImagesSubcommand},
	{"load-configuration-from-tftp", "", 0, 0,
		loadConfigurationFromTftpSubcommand},
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
	logger.Println("installation completed, copying logs and unmounting")
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
	runOnSignal(syscall.SIGHUP, func() {
		sighupHandler(logger)
	})
	rebootSemaphore := make(chan struct{}, 1)
	runOnSignal(syscall.SIGTSTP, func() {
		logger.Println("caught SIGTSTP: reboot blocked until SIGCONT")
		select {
		case rebootSemaphore <- struct{}{}:
		default:
		}
	})
	runOnSignal(syscall.SIGCONT, func() {
		logger.Println("caught SIGCONT: reboot unblocked")
		select {
		case <-rebootSemaphore:
		default:
		}
	})
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
	select {
	case rebootSemaphore <- struct{}{}:
	default:
		logger.Println("reboot blocked, waiting for SIGCONT")
		rebootSemaphore <- struct{}{}
	}
	<-rebootSemaphore
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

func runOnSignal(signum os.Signal, fn func()) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, signum)
	go func() {
		<-signalChannel
		fn()
	}()
}

func runSubcommand(args []string, logger log.DebugLogger) error {
	return runDaemon()
}

func sighupHandler(logger log.Logger) {
	logger.Printf("caught SIGHUP: re-execing with: %v\n", os.Args)
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		logger.Printf("error getting open file limit: %s\n", err)
		rlimit.Cur = 1024
	}
	for fd := 3; fd < int(rlimit.Cur); fd++ {
		syscall.CloseOnExec(fd)
	}
	if err := syscall.Exec(os.Args[0], os.Args, os.Environ()); err != nil {
		logger.Printf("unable to Exec: %s: %s\n", os.Args[0], err)
	}
}

func (w *logWriter) Write(p []byte) (int, error) {
	buffer := &bytes.Buffer{}
	fmt.Fprintf(buffer, "[%7.3f] ", time.Since(processStartTime).Seconds())
	buffer.Write(p)
	return w.writer.Write(buffer.Bytes())
}
