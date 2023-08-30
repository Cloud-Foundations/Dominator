package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/httpd"
	"github.com/Cloud-Foundations/Dominator/fleetmanager/hypervisors"
	"github.com/Cloud-Foundations/Dominator/fleetmanager/hypervisors/fsstorer"
	"github.com/Cloud-Foundations/Dominator/fleetmanager/rpcd"
	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/proxy"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

const (
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
)

var (
	checkTopology = flag.Bool("checkTopology", false,
		"If true, perform a one-time check, write to stdout and exit")
	ipmiPasswordFile = flag.String("ipmiPasswordFile", "",
		"Name of password file used to authenticate for IPMI requests")
	ipmiUsername = flag.String("ipmiUsername", "",
		"Name of user to authenticate as when making IPMI requests")
	topologyCheckInterval = flag.Duration("topologyCheckInterval",
		time.Minute, "Configuration check interval")
	portNum = flag.Uint("portNum", constants.FleetManagerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	stateDir = flag.String("stateDir", "/var/lib/fleet-manager",
		"Name of state directory")
	topologyDir = flag.String("topologyDir", "",
		"Name of local topology directory or directory in Git repository")
	topologyRepository = flag.String("topologyRepository", "",
		"URL of Git repository containing repository")
	variablesDir = flag.String("variablesDir", "",
		"Name of local variables directory or directory in Git repository")
)

func doCheck(logger log.DebugLogger) {
	topo, err := topology.LoadWithParams(topology.Params{
		Logger:       logger,
		TopologyDir:  *topologyDir,
		VariablesDir: *variablesDir,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.WriteWithIndent(os.Stdout, "    ", topo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if err := loadflags.LoadForDaemon("fleet-manager"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Parse()
	tricorder.RegisterFlags()
	logger := serverlogger.New("")
	srpc.SetDefaultLogger(logger)
	if *checkTopology {
		doCheck(logger)
	}
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		logger.Fatalln(err)
	}
	if err := proxy.New(logger); err != nil {
		logger.Fatalln(err)
	}
	if err := os.MkdirAll(*stateDir, dirPerms); err != nil {
		logger.Fatalf("Cannot create state directory: %s\n", err)
	}
	topologyChannel, err := topology.WatchWithParams(topology.WatchParams{
		Params: topology.Params{
			Logger:       logger,
			TopologyDir:  *topologyDir,
			VariablesDir: *variablesDir,
		},
		CheckInterval:      *topologyCheckInterval,
		LocalRepositoryDir: filepath.Join(*stateDir, "topology"),
		TopologyRepository: *topologyRepository,
	},
	)
	if err != nil {
		logger.Fatalf("Cannot watch for topology: %s\n", err)
	}
	storer, err := fsstorer.New(filepath.Join(*stateDir, "hypervisor-db"),
		logger)
	if err != nil {
		logger.Fatalf("Cannot create DB: %s\n", err)
	}
	hyperManager, err := hypervisors.New(hypervisors.StartOptions{
		IpmiPasswordFile: *ipmiPasswordFile,
		IpmiUsername:     *ipmiUsername,
		Logger:           logger,
		Storer:           storer,
	})
	if err != nil {
		logger.Fatalf("Cannot create hypervisors manager: %s\n", err)
	}
	rpcHtmlWriter, err := rpcd.Setup(hyperManager, logger)
	if err != nil {
		logger.Fatalf("Cannot start rpcd: %s\n", err)
	}
	webServer, err := httpd.StartServer(*portNum, logger)
	if err != nil {
		logger.Fatalf("Unable to create http server: %s\n", err)
	}
	webServer.AddHtmlWriter(hyperManager)
	webServer.AddHtmlWriter(rpcHtmlWriter)
	webServer.AddHtmlWriter(logger)
	for topology := range topologyChannel {
		logger.Println("Received new topology")
		webServer.UpdateTopology(topology)
		hyperManager.UpdateTopology(topology)
	}
}
