package net

import (
	"os"
	"path/filepath"
)

func listBridgePorts(bridge string) ([]string, error) {
	dir, err := os.Open(filepath.Join(sysClassNet, bridge, "brif"))
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.Readdirnames(0)
}
