package loadflags

import (
	"path/filepath"
)

func LoadForCli(progName string) error {
	return loadForCli(progName)
}

func LoadForDaemon(progName string) error {
	return loadFlags(filepath.Join("/etc", progName))
}
