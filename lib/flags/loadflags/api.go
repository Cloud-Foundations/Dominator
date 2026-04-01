package loadflags

import (
	"path/filepath"
)

func LoadForCli(progName string) error {
	registerVersionFlag(progName)
	return loadForCli(progName)
}

func LoadForDaemon(progName string) error {
	registerVersionFlag(progName)
	return loadFlags(filepath.Join("/etc", progName))
}
