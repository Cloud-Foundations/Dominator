package fsutil

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var losetupMutex sync.Mutex

func loopbackDelete(loopDevice string) error {
	losetupMutex.Lock()
	defer losetupMutex.Unlock()
	return exec.Command("losetup", "-d", loopDevice).Run()
}

func loopbackSetup(filename string) (string, error) {
	losetupMutex.Lock()
	defer losetupMutex.Unlock()
	cmd := exec.Command("losetup", "-fP", "--show", filename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}
