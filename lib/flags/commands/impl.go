package commands

import (
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func printCommands(writer io.Writer, commands []Command) {
	isSorted := sort.SliceIsSorted(commands, func(i, j int) bool {
		return commands[i].Command < commands[j].Command
	})
	if !isSorted {
		fmt.Fprintln(writer, "NOTE: COMMANDS ARE NOT SORTED!")
	}
	for _, command := range commands {
		if command.CmdFunc == nil {
			continue
		}
		if command.Args == "" {
			fmt.Fprintln(writer, " ", command.Command)
		} else {
			fmt.Fprintln(writer, " ", command.Command, command.Args)
		}
	}
	if !isSorted {
		fmt.Fprintln(writer, "NOTE: COMMANDS ARE NOT SORTED!")
	}
}

func runCommands(commands []Command, printUsage func(),
	logger log.DebugLogger) int {
	numCommandArgs := flag.NArg() - 1
	for _, command := range commands {
		if command.CmdFunc == nil {
			continue
		}
		if flag.Arg(0) == command.Command {
			if numCommandArgs < command.MinArgs ||
				(command.MaxArgs >= 0 &&
					numCommandArgs > command.MaxArgs) {
				printUsage()
				return 2
			}
			if err := command.CmdFunc(flag.Args()[1:], logger); err != nil {
				fmt.Fprintln(flag.CommandLine.Output(), err)
				return 1
			}
			return 0
		}
	}
	printUsage()
	return 2
}
