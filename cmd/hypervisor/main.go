package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/hypervisor/dhcpd"
	"github.com/Cloud-Foundations/Dominator/hypervisor/httpd"
	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/hypervisor/metadatad"
	"github.com/Cloud-Foundations/Dominator/hypervisor/rpcd"
	"github.com/Cloud-Foundations/Dominator/hypervisor/tftpbootd"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/commands"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

const (
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
)

var (
	dhcpServerOnBridgesOnly = flag.Bool("dhcpServerOnBridgesOnly", false,
		"If true, run the DHCP server on bridge interfaces only")
	imageServerHostname = flag.String("imageServerHostname", "localhost",
		"Hostname of image server")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	lockCheckInterval = flag.Duration("lockCheckInterval", 2*time.Second,
		"Interval between checks for lock timeouts")
	lockLogTimeout = flag.Duration("lockLogTimeout", 5*time.Second,
		"Timeout before logging that a lock has been held too long")
	networkBootImage = flag.String("networkBootImage", "pxelinux.0",
		"Name of boot image passed via DHCP option")
	objectCacheSize = flagutil.Size(10 << 30)
	portNum         = flag.Uint("portNum", constants.HypervisorPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	showVGA  = flag.Bool("showVGA", false, "If true, show VGA console")
	stateDir = flag.String("stateDir", "/var/lib/hypervisor",
		"Name of state directory")
	testMemoryAvailable = flag.Uint64("testMemoryAvailable", 0,
		"test if memory is allocatable and exit (units of MiB)")
	tftpbootImageStream = flag.String("tftpbootImageStream", "",
		"Name of default image stream for network booting")
	username = flag.String("username", "nobody",
		"Name of user to run VMs")
	volumeDirectories flagutil.StringList
)

func init() {
	flag.Var(&objectCacheSize, "objectCacheSize",
		"maximum size of object cache")
	flag.Var(&volumeDirectories, "volumeDirectories",
		"Comma separated list of volume directories. If empty, scan for space")
}

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w,
		"Usage: hypervisor [flags...] [run|stop|stop-vms-on-next-stop]")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"check-vms", "", 0, 0, checkVmsSubcommand},
	{"repair-vm-volume-allocations", "", 0, 0,
		repairVmVolumeAllocationsSubcommand},
	{"run", "", 0, 0, runSubcommand},
	{"stop", "", 0, 0, stopSubcommand},
	{"stop-vms-on-next-stop", "", 0, 0, stopVmsOnNextStopSubcommand},
}

func processCommand(args []string) {
	if len(args) < 1 {
		runSubcommand(nil, nil)
	}
	os.Exit(commands.RunCommands(subcommands, printUsage, nil))
}

func main() {
	if err := loadflags.LoadForDaemon("hypervisor"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Usage = printUsage
	flag.Parse()
	processCommand(flag.Args())
}

func run() {
	if *testMemoryAvailable > 0 {
		nBytes := *testMemoryAvailable << 20
		mem := make([]byte, nBytes)
		for pos := uint64(0); pos < nBytes; pos += 4096 {
			mem[pos] = 0
		}
		os.Exit(0)
	}
	tricorder.RegisterFlags()
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "Must run the Hypervisor as root")
		os.Exit(1)
	}
	logger := serverlogger.New("")
	srpc.SetDefaultLogger(logger)
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		logger.Fatalln(err)
	}
	if err := os.MkdirAll(*stateDir, dirPerms); err != nil {
		logger.Fatalf("Cannot create state directory: %s\n", err)
	}
	bridges, bridgeMap, err := net.ListBroadcastInterfaces(
		net.InterfaceTypeBridge, logger)
	if err != nil {
		logger.Fatalf("Cannot list bridges: %s\n", err)
	}
	dhcpInterfaces := make([]string, 0, len(bridges))
	vlanIdToBridge := make(map[uint]string)
	for _, bridge := range bridges {
		vlanId, err := net.GetBridgeVlanId(bridge.Name)
		if err != nil {
			logger.Fatalf("Cannot get VLAN Id for bridge: %s: %s\n",
				bridge.Name, err)
			continue
		}
		if vlanId < 0 {
			if len(bridges) > 1 {
				logger.Printf("Bridge: %s has no EtherNet port, ignoring\n",
					bridge.Name)
				continue
			}
			logger.Printf(
				"Bridge: %s has no EtherNet port but is the only bridge, using in hope\n",
				bridge.Name)
			vlanId = 0
		}
		if *dhcpServerOnBridgesOnly {
			dhcpInterfaces = append(dhcpInterfaces, bridge.Name)
		}
		if !strings.HasPrefix(bridge.Name, "br@") {
			vlanIdToBridge[uint(vlanId)] = bridge.Name
			logger.Printf("Bridge: %s, VLAN Id: %d\n", bridge.Name, vlanId)
		}
	}
	dhcpServer, err := dhcpd.New(dhcpInterfaces,
		filepath.Join(*stateDir, "dynamic-leases.json"), logger)
	if err != nil {
		logger.Fatalf("Cannot start DHCP server: %s\n", err)
	}
	if err := dhcpServer.SetNetworkBootImage(*networkBootImage); err != nil {
		logger.Fatalf("Cannot set NetworkBootImage name: %s\n", err)
	}
	imageServerAddress := fmt.Sprintf("%s:%d",
		*imageServerHostname, *imageServerPortNum)
	tftpbootServer, err := tftpbootd.New(imageServerAddress,
		*tftpbootImageStream, logger)
	if err != nil {
		logger.Fatalf("Cannot start tftpboot server: %s\n", err)
	}
	managerObj, err := manager.New(manager.StartOptions{
		BridgeMap:          bridgeMap,
		DhcpServer:         dhcpServer,
		ImageServerAddress: imageServerAddress,
		LockCheckInterval:  *lockCheckInterval,
		LockLogTimeout:     *lockLogTimeout,
		Logger:             logger,
		ObjectCacheBytes:   uint64(objectCacheSize),
		ShowVgaConsole:     *showVGA,
		StateDir:           *stateDir,
		Username:           *username,
		VlanIdToBridge:     vlanIdToBridge,
		VolumeDirectories:  volumeDirectories,
	})
	if err != nil {
		logger.Fatalf("Cannot start hypervisor: %s\n", err)
	}
	if err := listenForControl(managerObj, logger); err != nil {
		logger.Fatalf("Cannot listen for control: %s\n", err)
	}
	httpd.AddHtmlWriter(managerObj)
	if len(bridges) < 1 {
		logger.Println("No bridges found: entering log-only mode")
	} else {
		httpd.AddHtmlWriter(dhcpServer)
		rpcHtmlWriter, err := rpcd.Setup(managerObj, dhcpServer, tftpbootServer,
			logger)
		if err != nil {
			logger.Fatalf("Cannot start rpcd: %s\n", err)
		}
		httpd.AddHtmlWriter(rpcHtmlWriter)
	}
	httpd.AddHtmlWriter(logger)
	err = metadatad.StartServer(*portNum, bridges, managerObj, logger)
	if err != nil {
		logger.Fatalf("Cannot start metadata server: %s\n", err)
	}
	if err := httpd.StartServer(*portNum, managerObj, false); err != nil {
		logger.Fatalf("Unable to create http server: %s\n", err)
	}
}

func runSubcommand(args []string, logger log.DebugLogger) error {
	run()
	return fmt.Errorf("unexpected return from run()")
}
