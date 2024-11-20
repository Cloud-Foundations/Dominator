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
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

var (
	cpuPercent = flag.Uint("cpuPercent", 0,
		"CPU speed as percentage of capacity (default 50)")
	disruptionManagerUrl = flag.String("disruptionManagerUrl", "",
		"URL of Disruption Manager endpoint")
	domHostname = flag.String("domHostname", "localhost",
		"Hostname of dominator")
	domPortNum = flag.Uint("domPortNum", constants.DominatorPortNumber,
		"Port number of dominator")
	locationsToMatch  flagutil.StringList
	mdbServerHostname = flag.String("mdbServerHostname", "",
		"Hostname of MDB server (default same as domHostname)")
	mdbServerPortNum = flag.Uint("mdbServerPortNum",
		constants.SimpleMdbServerPortNumber,
		"Port number of MDB server")
	networkSpeedPercent = flag.Uint("networkSpeedPercent",
		constants.DefaultNetworkSpeedPercent,
		"Network speed as percentage of capacity")
	pauseDuration = flag.Duration("pauseDuration", time.Hour,
		"Duration to pause updates for sub")
	scanExcludeList  flagutil.StringList = constants.ScanExcludeList
	scanSpeedPercent                     = flag.Uint("scanSpeedPercent",
		constants.DefaultScanSpeedPercent,
		"Scan speed as percentage of capacity")
	statusesToMatch flagutil.StringList
	subsList        = flag.String("subsList", "",
		"Name of file containing list of subs")
	tagsToMatch tags.MatchTags

	dominatorSrpcClient *srpc.Client
)

func init() {
	flag.Var(&locationsToMatch, "locationsToMatch",
		"Sub locations to match when listing")
	flag.Var(&scanExcludeList, "scanExcludeList",
		"Comma separated list of patterns to exclude from scanning")
	flag.Var(&statusesToMatch, "statusesToMatch",
		"Sub statuses to match when listing")
	flag.Var(&tagsToMatch, "tagsToMatch", "Tags to match when listing")
}

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "Usage: domtool [flags...] command")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"clear-safety-shutoff", "sub", 1, 1, clearSafetyShutoffSubcommand},
	{"configure-subs", "", 0, 0, configureSubsSubcommand},
	{"disable-updates", "reason", 1, 1, disableUpdatesSubcommand},
	{"disruption-cancel", "sub", 1, 1, disruptionCancelSubcommand},
	{"disruption-check", "sub", 1, 1, disruptionCheckSubcommand},
	{"disruption-request", "sub", 1, 1, disruptionRequestSubcommand},
	{"enable-updates", "reason", 1, 1, enableUpdatesSubcommand},
	{"force-disruptive-update", "sub", 1, 1, forceDisruptiveUpdateSubcommand},
	{"get-default-image", "", 0, 0, getDefaultImageSubcommand},
	{"get-info-for-subs", "", 0, 0, getInfoForSubsSubcommand},
	{"get-machine-from-mdb", "sub", 1, 1, getMachineMdbSubcommand},
	{"get-mdb", "", 0, 0, getMdbSubcommand},
	{"get-mdb-updates", "", 0, 0, getMdbUpdatesSubcommand},
	{"get-subs-configuration", "", 0, 0, getSubsConfigurationSubcommand},
	{"list-subs", "", 0, 0, listSubsSubcommand},
	{"pause-sub-updates", "sub reason", 2, 2, pauseSubUpdatesSubcommand},
	{"resume-sub-updates", "sub", 1, 1, resumeSubUpdatesSubcommand},
	{"set-default-image", "", 1, 1, setDefaultImageSubcommand},
}

func getClient() *srpc.Client {
	if dominatorSrpcClient != nil {
		return dominatorSrpcClient
	}
	clientName := fmt.Sprintf("%s:%d", *domHostname, *domPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dialing: %s: %s\n", clientName, err)
		os.Exit(1)
	}
	dominatorSrpcClient = client
	return dominatorSrpcClient
}

func doMain() int {
	if err := loadflags.LoadForCli("domtool"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Usage = printUsage
	flag.Parse()
	if flag.NArg() < 1 {
		printUsage()
		return 2
	}
	logger := cmdlogger.New()
	srpc.SetDefaultLogger(logger)
	if err := setupclient.SetupTls(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return commands.RunCommands(subcommands, printUsage, logger)
}

func main() {
	os.Exit(doMain())
}
