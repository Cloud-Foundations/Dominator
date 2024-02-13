//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/imagebuilder/builder"
	"github.com/Cloud-Foundations/Dominator/imagebuilder/httpd"
	"github.com/Cloud-Foundations/Dominator/imagebuilder/logarchiver"
	"github.com/Cloud-Foundations/Dominator/imagebuilder/rpcd"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupserver"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

const (
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
)

var (
	buildLogDir = flag.String("buildLogDir", "/var/log/imaginator/builds",
		"Name of directory to write build logs to")
	buildLogQuota    = flagutil.Size(100 << 20)
	configurationUrl = flag.String("configurationUrl",
		"file:///etc/imaginator/conf.json", "URL containing configuration")
	imageServerHostname = flag.String("imageServerHostname", "localhost",
		"Hostname of image server")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	imageRebuildInterval = flag.Duration("imageRebuildInterval", time.Hour,
		"time between automatic rebuilds of images")
	maximumExpirationDuration = flag.Duration("maximumExpirationDuration",
		24*time.Hour, "Maximum expiration time for regular users")
	maximumExpirationDurationPrivileged = flag.Duration(
		"maximumExpirationDurationPrivileged", 730*time.Hour,
		"Maximum expiration time for privileged users")
	minimumExpirationDuration = flag.Duration("minimumExpirationDuration",
		15*time.Minute,
		"Minimum permitted expiration duration")
	portNum = flag.Uint("portNum", constants.ImaginatorPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	slaveDriverConfigurationFile = flag.String("slaveDriverConfigurationFile",
		"", "Name of configuration file for slave builders")
	stateDir = flag.String("stateDir", "/var/lib/imaginator",
		"Name of state directory")
	variablesFile = flag.String("variablesFile", "",
		"A JSON encoded file containing special variables (i.e. secrets)")
)

func init() {
	flag.Var(&buildLogQuota, "buildLogQuota",
		"Build log quota. If exceeded, old logs are deleted")
}

func main() {
	if err := loadflags.LoadForDaemon("imaginator"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Parse()
	tricorder.RegisterFlags()
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "Must run the Image Builder as root")
		os.Exit(1)
	}
	logger := serverlogger.New("")
	srpc.SetDefaultLogger(logger)
	if umask := syscall.Umask(022); umask != 022 {
		// Since we can't cleanly fix umask for all threads, fail instead.
		logger.Fatalf("Umask must be 022, not 0%o\n", umask)
	}
	params := setupserver.Params{Logger: logger}
	if err := setupserver.SetupTlsWithParams(params); err != nil {
		logger.Fatalln(err)
	}
	if err := os.MkdirAll(*stateDir, dirPerms); err != nil {
		logger.Fatalf("Cannot create state directory: %s\n", err)
	}
	slaveDriver, createSlaveTimeout, err := createSlaveDriver(logger)
	if err != nil {
		logger.Fatalf("Error starting slave driver: %s\n", err)
	}
	var buildLogArchiver logarchiver.BuildLogger
	if *buildLogDir != "" && buildLogQuota > 1<<20 {
		buildLogArchiver, err = logarchiver.New(
			logarchiver.BuildLogArchiveOptions{
				Quota:  uint64(buildLogQuota),
				Topdir: *buildLogDir,
			},
			logarchiver.BuildLogArchiveParams{
				Logger: logger,
			},
		)
		if err != nil {
			logger.Fatalf("Error starting build log archiver: %s\n", err)
		}
	}
	builderObj, err := builder.LoadWithOptionsAndParams(
		builder.BuilderOptions{
			ConfigurationURL:     *configurationUrl,
			CreateSlaveTimeout:   createSlaveTimeout,
			ImageRebuildInterval: *imageRebuildInterval,
			ImageServerAddress: fmt.Sprintf("%s:%d",
				*imageServerHostname, *imageServerPortNum),
			MaximumExpirationDuration:           *maximumExpirationDuration,
			MaximumExpirationDurationPrivileged: *maximumExpirationDurationPrivileged,
			MinimumExpirationDuration:           *minimumExpirationDuration,
			StateDirectory:                      *stateDir,
			VariablesFile:                       *variablesFile,
		},
		builder.BuilderParams{
			BuildLogArchiver: buildLogArchiver,
			Logger:           logger,
			SlaveDriver:      slaveDriver,
		})
	if err != nil {
		logger.Fatalf("Cannot start builder: %s\n", err)
	}
	rpcHtmlWriter, err := rpcd.Setup(builderObj, logger)
	if err != nil {
		logger.Fatalf("Cannot start builder: %s\n", err)
	}
	httpd.AddHtmlWriter(builderObj)
	if slaveDriver != nil {
		httpd.AddHtmlWriter(slaveDriver)
	}
	httpd.AddHtmlWriter(rpcHtmlWriter)
	httpd.AddHtmlWriter(logger)
	err = httpd.StartServerWithOptionsAndParams(
		httpd.Options{
			PortNumber: *portNum,
		},
		httpd.Params{
			Builder:          builderObj,
			BuildLogReporter: buildLogArchiver,
			Logger:           logger,
		},
	)
	if err != nil {
		logger.Fatalf("Unable to create http server: %s\n", err)
	}
}
