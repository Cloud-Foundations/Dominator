package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

var (
	datacentre = flag.String("datacentre", "",
		"Datacentre to limit results to (may not be supported by all drivers)")
	debug         = flag.Bool("debug", false, "Deprecated")
	fetchInterval = flag.Uint("fetchInterval", 59,
		"Interval between fetches from the MDB source, in seconds")
	hostnamesExcludeFile = flag.String("hostnamesExcludeFile", "",
		"A file containing a list of hostnames to exclude")
	hostnamesIncludeFile = flag.String("hostnamesIncludeFile", "",
		"A file containing a list of hostnames to include")
	hostnameRegex = flag.String("hostnameRegex", ".*",
		"A regular expression to match the desired hostnames, leading ! inverts")
	mdbFile = flag.String("mdbFile", constants.DefaultMdbFile,
		"Name of file to write filtered MDB data to")
	portNum = flag.Uint("portNum", constants.SimpleMdbServerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	sourcesFile = flag.String("sourcesFile", "/var/lib/mdbd/mdb.sources.list",
		"Name of file list of driver url pairs")
	stateDir = flag.String("stateDir", "/var/lib/mdbd",
		"Name of state directory")
	pidfile = flag.String("pidfile", "",
		"Name of file to write my PID to")
	variablesFile = flag.String("variablesFile", "",
		"A JSON encoded file containing configuration variables")
)

func printUsage() {
	fmt.Fprintln(os.Stderr,
		"Usage: mdbd [flags...]")
	fmt.Fprintln(os.Stderr, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Drivers:")
	fmt.Fprintln(os.Stderr,
		"  aws: region account")
	fmt.Fprintln(os.Stderr,
		"    Query Amazon AWS")
	fmt.Fprintln(os.Stderr,
		"    region:  a datacentre like 'us-east-1'")
	fmt.Fprintln(os.Stderr,
		"    account: the profile to use out of ~/.aws/credentials which")
	fmt.Fprintln(os.Stderr,
		"             contains the amazon aws credentials. For additional")
	fmt.Fprintln(os.Stderr,
		"             information see:")
	fmt.Fprintln(os.Stderr,
		"             http://docs.aws.amazon.com/sdk-for-go/latest/v1/developerguide/sdkforgo-dg.pdf")
	fmt.Fprintln(os.Stderr,
		"  aws-filtered: targets filter-tags-file")
	fmt.Fprintln(os.Stderr,
		"    Query Amazon AWS")
	fmt.Fprintln(os.Stderr,
		"    targets:          a list of targets, i.e. 'prod,us-east-1;dev,us-east-1'")
	fmt.Fprintln(os.Stderr,
		"    filter-tags-file: a JSON file of tags to filter for")
	fmt.Fprintln(os.Stderr,
		"  aws-local")
	fmt.Fprintln(os.Stderr,
		"    Query Amazon AWS for all acccounts for the local region")
	fmt.Fprintln(os.Stderr,
		"  cis: url")
	fmt.Fprintln(os.Stderr,
		"    url: Cloud Intelligence Service endpoint search query")
	fmt.Fprintln(os.Stderr,
		"  ds.host.fqdn: url")
	fmt.Fprintln(os.Stderr,
		"    url: URL which yields JSON with map of map of hosts with fqdn entries")
	fmt.Fprintln(os.Stderr,
		"  fleet-manager: manager-hostname [location]")
	fmt.Fprintln(os.Stderr,
		"    Query Fleet Manager")
	fmt.Fprintln(os.Stderr,
		"    manager-hostname: hostname of the Fleet Manager")
	fmt.Fprintln(os.Stderr,
		"    location:         optional location to limit query to")
	fmt.Fprintln(os.Stderr,
		"  hostlist: url [required-image [planned-image]]")
	fmt.Fprintln(os.Stderr,
		"    url:            URL which yields a list of machine hostnames, one per line")
	fmt.Fprintln(os.Stderr,
		"    required-image: optional required image for machines")
	fmt.Fprintln(os.Stderr,
		"    planned-image:  optional planned image for machines")
	fmt.Fprintln(os.Stderr,
		"  hypervisor")
	fmt.Fprintln(os.Stderr,
		"    Query Hypervisor on this machine")
	fmt.Fprintln(os.Stderr,
		"  json: url [prefix]")
	fmt.Fprintln(os.Stderr,
		"    url:      URL which yields a JSON-formatted list of machines and tags")
	fmt.Fprintln(os.Stderr,
		"    prefix:   optional prefix to add to Location fields")
	fmt.Fprintln(os.Stderr,
		"  text: url")
	fmt.Fprintln(os.Stderr,
		"    url: URL which yields lines. Each line contains:")
	fmt.Fprintln(os.Stderr,
		"         host [required-image [planned-image]]")
	fmt.Fprintln(os.Stderr,
		"  topology: url [location [prefix]]")
	fmt.Fprintln(os.Stderr,
		"    Load Topology (only one permitted)")
	fmt.Fprintln(os.Stderr,
		"    url:      directory or Git URL containing the Topology")
	fmt.Fprintln(os.Stderr,
		"    location: optional subdirectory containing the Topology")
	fmt.Fprintln(os.Stderr,
		"    prefix:   optional prefix to add to Location fields")
}

type driver struct {
	name      string
	minArgs   int
	maxArgs   int
	setupFunc makeGeneratorFunc
}

var drivers = []driver{
	{"aws", 2, 2, newAwsGenerator},
	{"aws-filtered", 2, 2, newAwsFilteredGenerator},
	{"aws-local", 0, 0, newAwsLocalGenerator},
	{"cis", 1, 1, newCisGenerator},
	{"ds.host.fqdn", 1, 1, newDsHostFqdnGenerator},
	{"fleet-manager", 1, 2, newFleetManagerGenerator},
	{"hostlist", 1, 3, newHostlistGenerator},
	{"hypervisor", 0, 0, newHypervisorGenerator},
	{"json", 1, 2, newJsonGenerator},
	{"text", 1, 1, newTextGenerator},
	{"topology", 1, 3, newTopologyGenerator},
}

func gracefulCleanup() {
	os.Remove(*pidfile)
	os.Exit(1)
}

func writePidfile() {
	file, err := os.Create(*pidfile)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintln(file, os.Getpid())
}

func handleSignals(logger log.Logger) {
	if *pidfile == "" {
		return
	}
	sigtermChannel := make(chan os.Signal)
	signal.Notify(sigtermChannel, syscall.SIGTERM, syscall.SIGINT)
	writePidfile()
	go func() {
		for {
			select {
			case <-sigtermChannel:
				gracefulCleanup()
			}
		}
	}()
}

func showErrorAndDie(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

func main() {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "Do not run the MDB daemon as root")
		os.Exit(1)
	}
	if err := loadflags.LoadForDaemon("mdbd"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Usage = printUsage
	flag.Parse()
	tricorder.RegisterFlags()
	logger := serverlogger.New("")
	if *debug { // Backwards compatibility.
		logger.SetLevel(0)
	}
	srpc.SetDefaultLogger(logger)
	var variables map[string]string
	if *variablesFile != "" {
		if err := json.ReadFromFile(*variablesFile, &variables); err != nil {
			showErrorAndDie(err)
		}
	}
	// We have to have inputs.
	if *sourcesFile == "" {
		printUsage()
		os.Exit(2)
	}
	params := setupserver.Params{ClientOnly: true, Logger: logger}
	setupserver.SetupTlsWithParams(params)
	handleSignals(logger)
	readerChannel := fsutil.WatchFile(*sourcesFile, logger)
	file, err := os.Open(*sourcesFile)
	if err != nil {
		showErrorAndDie(err)
	}
	(<-readerChannel).Close()
	eventChannel := make(chan struct{}, 1)
	waitGroup := &sync.WaitGroup{}
	generatorParams := makeGeneratorParams{
		eventChannel: eventChannel,
		logger:       logger,
		waitGroup:    waitGroup,
	}
	generators, err := setupGenerators(file, drivers, generatorParams,
		variables)
	file.Close()
	if err != nil {
		showErrorAndDie(err)
	}
	httpSrv, err := startHttpServer(*portNum, variables, generators)
	if err != nil {
		showErrorAndDie(err)
	}
	httpSrv.AddHtmlWriter(logger)
	startHostsExcludeReader(*hostnamesExcludeFile, eventChannel, waitGroup,
		logger)
	startHostsIncludeReader(*hostnamesIncludeFile, eventChannel, waitGroup,
		logger)
	// Wait a minute for any asynronous generators to yield first data.
	waitTimer := time.NewTimer(time.Minute)
	waitChannel := make(chan struct{}, 1)
	go func() {
		waitGroup.Wait()
		waitChannel <- struct{}{}
	}()
	select {
	case <-waitChannel:
		logger.Println("Asynchronous generators completed initial generation")
	case <-waitTimer.C:
		logger.Println("Timed out waiting for initial data")
	}
	rpcd := startRpcd(logger)
	go runDaemon(generators, eventChannel, *mdbFile, *hostnameRegex,
		*datacentre, *fetchInterval, func(old, new *mdb.Mdb) {
			rpcd.pushUpdateToAll(old, new)
			httpSrv.UpdateMdb(new)
		},
		logger)
	<-readerChannel
	fsutil.WatchFileStop()
	if err := syscall.Exec(os.Args[0], os.Args, os.Environ()); err != nil {
		logger.Printf("Unable to Exec:%s: %s\n", os.Args[0], err)
	}
}
