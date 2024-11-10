package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
)

var (
	maximimPermittedDuration = flag.Duration("maximimPermittedDuration",
		time.Hour,
		"Maximum time disruption will be permitted after last request")
	portNum = flag.Uint("portNum", constants.DisruptionManagerPortNumber,
		"Port number to allocate and listen on for HTTP/RPC")
	stateDir = flag.String("stateDir", "/var/lib/disruption-manager",
		"Name of state directory")
)

func showErrorAndDie(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

func main() {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "Do not run the Disruption Manager as root")
		os.Exit(1)
	}
	if err := loadflags.LoadForDaemon("disruption-manager"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.Parse()
	tricorder.RegisterFlags()
	logger := serverlogger.New("")
	dm, err := newDisruptionManager(filepath.Join(*stateDir, "state.json"),
		*maximimPermittedDuration, logger)
	if err != nil {
		logger.Fatalf("Unable to create Disruption Manager\n", err)
	}
	webServer, err := startHttpServer(dm, logger)
	if err != nil {
		logger.Fatalf("Unable to create http server: %s\n", err)
	}
	webServer.AddHtmlWriter(logger)
	if err := webServer.serve(*portNum); err != nil {
		logger.Fatalf("Unable to start http server: %s\n", err)
	}
}
