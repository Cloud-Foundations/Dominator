package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/commands"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/log/cmdlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupclient"
)

var (
	alwaysShowBuildLog = flag.Bool("alwaysShowBuildLog", false,
		"If true, show build log even for successful builds")
	bindMounts      flagutil.StringList
	digraphExcludes flagutil.StringList
	digraphIncludes flagutil.StringList
	disableFor      = flag.Duration("disableFor", 5*time.Minute,
		"How long to disable")
	expiresIn = flag.Duration("expiresIn", time.Hour,
		"How long before the image expires (auto deletes)")
	imaginatorHostname = flag.String("imaginatorHostname", "localhost",
		"Hostname of image build server")
	imaginatorPortNum = flag.Uint("imaginatorPortNum",
		constants.ImaginatorPortNumber,
		"Port number of image build server")
	imageFilename = flag.String("imageFilename", "",
		"Name of file to write image to")
	imageServerHostname = flag.String("imageServerHostname", "localhost",
		"Hostname of image server")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	maxSourceAge = flag.Duration("maxSourceAge", time.Hour,
		"Maximum age of a source image before it is rebuilt")
	rawSize      flagutil.Size
	showFetchLog = flag.Bool("showFetchLog", false,
		"If true, show fetch log when getting directed graph")
	variablesFilename = flag.String("variablesFilename", "",
		"Name of file to read variables from for local builds")

	minimumExpiration = 5 * time.Minute
)

func init() {
	flag.Var(&bindMounts, "bindMounts",
		"Comma separated list of directories to bind mount into build workspace")
	flag.Var(&digraphExcludes, "digraphExcludes",
		"Comma separated list of excludes when generating digraph")
	flag.Var(&digraphIncludes, "digraphIncludes",
		"Comma separated list of includes when generating digraph")
	flag.Var(&rawSize, "rawSize", "Size of RAW file to create")
}

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "Usage: builder-tool [flags...] command [args...]")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"build-from-manifest", "manifestDir stream-name", 2, 2,
		buildFromManifestSubcommand},
	{"build-image", "stream-name [git-branch]", 1, 2, buildImageSubcommand},
	{"build-raw-from-manifest", "manifestDir rawFile", 2, 2,
		buildRawFromManifestSubcommand},
	{"build-tree-from-manifest", "manifestDir", 1, 1,
		buildTreeFromManifestSubcommand},
	{"disable-auto-builds", "", 0, 0, disableAutoBuildsSubcommand},
	{"disable-build-requests", "", 0, 0, disableBuildRequestsSubcommand},
	{"get-dependencies", "", 0, 0, getDependenciesSubcommand},
	{"get-digraph", "", 0, 0, getDirectedGraphSubcommand},
	{"process-manifest", "manifestDir rootDir", 2, 2,
		processManifestSubcommand},
	{"replace-idle-slaves", "", 0, 0, replaceIdleSlavesSubcommand},
}

var imaginatorSrpcClient *srpc.Client
var imageServerSrpcClient *srpc.Client

func getImaginatorClient() *srpc.Client {
	if imaginatorSrpcClient == nil {
		var err error
		clientName := fmt.Sprintf("%s:%d",
			*imaginatorHostname, *imaginatorPortNum)
		imaginatorSrpcClient, err = srpc.DialHTTP("tcp", clientName, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error dialing: %s: %s\n", clientName, err)
			os.Exit(1)
		}
	}
	return imaginatorSrpcClient
}

func getImageServerClient() *srpc.Client {
	if imageServerSrpcClient == nil {
		var err error
		clientName := fmt.Sprintf("%s:%d",
			*imageServerHostname, *imageServerPortNum)
		imageServerSrpcClient, err = srpc.DialHTTP("tcp", clientName, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error dialing: %s: %s\n", clientName, err)
			os.Exit(1)
		}
	}
	return imageServerSrpcClient
}

func doMain() int {
	if err := loadflags.LoadForCli("builder-tool"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Usage = printUsage
	flag.Parse()
	if flag.NArg() < 1 {
		printUsage()
		return 3
	}
	if *expiresIn > 0 && *expiresIn < minimumExpiration {
		fmt.Fprintf(os.Stderr, "Minimum expiration: %s\n", minimumExpiration)
		return 2
	}
	logger := cmdlogger.New()
	srpc.SetDefaultLogger(logger)
	if err := setupclient.SetupTls(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	os.Unsetenv("LANG")
	return commands.RunCommands(subcommands, printUsage, logger)
}

func main() {
	os.Exit(doMain())
}
