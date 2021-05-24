package mounts

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const (
	procMounts = "/proc/mounts"
)

func getMountTable() (*MountTable, error) {
	file, err := os.Open(procMounts)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	table := &MountTable{}
	for scanner.Scan() {
		line := scanner.Text()
		var junk string
		var entry MountEntry
		nScanned, err := fmt.Sscanf(line, "%s %s %s %s %s",
			&entry.Device, &entry.MountPoint, &entry.Type, &entry.Options,
			&junk)
		if err != nil {
			return nil, err
		}
		if nScanned < 4 {
			return nil, fmt.Errorf("only read %d values from %s",
				nScanned, line)
		}
		table.Entries = append(table.Entries, &entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return table, nil
}

func (mt *MountTable) findEntry(path string) *MountEntry {
	var lastMatch *MountEntry
	var lastLength int
	for _, entry := range mt.Entries {
		length := len(entry.MountPoint)
		if strings.HasPrefix(path, entry.MountPoint) && length > lastLength {
			lastMatch = entry
			lastLength = length
		}
	}
	return lastMatch
}
