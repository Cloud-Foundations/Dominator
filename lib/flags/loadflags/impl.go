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

func registerVersionFlag(name string) {
	flag.BoolFunc("version", "Print version information and exit", func(string) error {
		fmt.Println(version.Get().Full(name))
		os.Exit(0)
		return nil
	})
}

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
	if err := loadFlags(filepath.Join(systemDir, progName)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
	}
	return loadFlags(
		filepath.Join(os.Getenv("HOME"), ".config", progName))
}
