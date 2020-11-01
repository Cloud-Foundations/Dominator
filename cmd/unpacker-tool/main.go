package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/commands"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/log/cmdlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupclient"
)

var (
	filename = flag.String("filename", "",
		"Name of file to write raw image data to")
	imageUnpackerHostname = flag.String("imageUnpackerHostname", "localhost",
		"Hostname of image-unpacker server")
	imageUnpackerPortNum = flag.Uint("imageUnpackerPortNum",
		constants.ImageUnpackerPortNumber,
		"Port number of image-unpacker server")

	unpackerSrpcClient *srpc.Client
)

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "Usage: unpacker-tool [flags...] add-device [args...]")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"add-device", "DeviceId command ...", 2, -1, addDeviceSubcommand},
	{"associate", "stream-name DeviceId", 2, 2, associateSubcommand},
	{"claim-device", "DeviceId DeviceName", 2, 2, claimDeviceSubcommand},
	{"export-image", "stream-name type destination", 3, 3,
		exportImageSubcommand},
	{"forget-stream", "stream-name ", 1, 1, forgetStreamSubcommand},
	{"get-raw", "stream-name ", 1, 1, getRawSubcommand},
	{"get-status", "", 0, 0, getStatusSubcommand},
	{"get-device-for-stream", "stream-name", 1, 1,
		getDeviceForStreamSubcommand},
	{"prepare-for-capture", "stream-name", 1, 1, prepareForCaptureSubcommand},
	{"prepare-for-copy", "stream-name", 1, 1, prepareForCopySubcommand},
	{"prepare-for-unpack", "stream-name", 1, 1, prepareForUnpackSubcommand},
	{"remove-device", "DeviceId", 1, 1, removeDeviceSubcommand},
	{"unpack-image", "stream-name image-leaf-name", 2, 2,
		unpackImageSubcommand},
}

func getClient() *srpc.Client {
	return unpackerSrpcClient
}

func doMain() int {
	if err := loadflags.LoadForCli("unpacker-tool"); err != nil {
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
	if err := setupclient.SetupTls(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	clientName := fmt.Sprintf("%s:%d",
		*imageUnpackerHostname, *imageUnpackerPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dialing\t%s\n", err)
		return 1
	}
	unpackerSrpcClient = client
	return commands.RunCommands(subcommands, printUsage, logger)
}

func main() {
	os.Exit(doMain())
}
