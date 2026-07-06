package loadflags

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/version"
)

const systemDir = "/etc/config"

func loadFlags(dirname string) error {
	err := loadFlagsFromFile(filepath.Join(dirname, "flags.default"))
	if err != nil {
		return err
	}
	return loadFlagsFromFile(filepath.Join(dirname, "flags.extra"))
}

func loadFlagsFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) < 1 {
			continue
		}
		if line[0] == '#' || line[0] == ';' {
			continue
		}
		splitLine := strings.SplitN(line, "=", 2)
		if len(splitLine) < 2 {
			return errors.New("bad line, cannot split name from value: " + line)
		}
		name := strings.TrimSpace(splitLine[0])
		if strings.Count(name, " ") != 0 {
			return errors.New("bad line, name has whitespace: " + line)
		}
		value := strings.TrimSpace(splitLine[1])
		if err := flag.CommandLine.Set(name, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func loadForCli(progName string) error {
	registerVersionFlag(progName)
	if err := loadFlags(filepath.Join(systemDir, progName)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
	}
	return loadFlags(
		filepath.Join(os.Getenv("HOME"), ".config", progName))
}

func loadForDaemon(progName string) error {
	registerVersionFlag(progName)
	return loadFlags(filepath.Join("/etc", progName))
}

func registerVersionFlag(name string) {
	flag.BoolFunc("version", "Print version information and exit",
		func(string) error {
			info := version.Get()
			out := flag.CommandLine.Output()
			fmt.Fprintf(out, "%s %s\n", name, info.Version)
			fmt.Fprintf(out, "  Commit: %s\n", info.GitCommit)
			if info.IsFork {
				fmt.Fprintf(out, "  Origin: %s\n", info.GitOrigin)
			}
			if info.GitBranch != "master" {
				fmt.Fprintf(out, "  Branch: %s\n", info.GitBranch)
			}
			fmt.Fprintf(out, "  Behind: %s\n", info.Behind())
			fmt.Fprintf(out, "  Built:  %s\n", info.BuildDate)
			fmt.Fprintf(out, "  Go:     %s\n", info.GoVersion)
			os.Exit(0)
			return nil
		})
}
