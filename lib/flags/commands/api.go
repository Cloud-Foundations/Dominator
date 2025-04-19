package commands

import (
	"flag"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type CommandFunc func([]string, log.DebugLogger) error

type Command struct {
	Command string
	Args    string
	MinArgs int
	MaxArgs int
	CmdFunc CommandFunc
}

var (
	cpuProfileFilename = flag.String("cpuProfileFilename", "",
		"Save a CPU profile of the subcommand to the specified file")
)

func PrintCommands(writer io.Writer, commands []Command) {
	printCommands(writer, commands)
}

func RunCommands(commands []Command, printUsage func(),
	logger log.DebugLogger) int {
	return runCommands(commands, printUsage, logger)
}
