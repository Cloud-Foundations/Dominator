package fsutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
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

func loopbackSetupAndWaitForPartition(filename, partition string,
	timeout time.Duration, logger log.DebugLogger) (string, error) {
	if timeout < 0 || timeout > time.Hour {
		timeout = time.Hour
	}
	loopDevice, err := LoopbackSetup(filename)
	if err != nil {
		return "", err
	}
	doDelete := true
	defer func() {
		if doDelete {
			LoopbackDelete(loopDevice)
		}
	}()
	// Probe for partition device because it might not be immediately available.
	// Need to open rather than just test for inode existance, because an
	// Open(2) is what may be needed to trigger dynamic device node creation.
	partitionDevice := loopDevice + partition
	sleeper := backoffdelay.NewExponential(time.Millisecond,
		100*time.Millisecond, 2)
	startTime := time.Now()
	stopTime := startTime.Add(timeout)
	for count := 0; time.Until(stopTime) >= 0; count++ {
		if file, err := os.Open(partitionDevice); err == nil {
			fi, err := file.Stat()
			file.Close()
			if err != nil {
				return "", err
			}
			if fi.Mode()&os.ModeDevice == 0 {
				return "", fmt.Errorf("%s is not a block device, mode: %s",
					partitionDevice, fi.Mode())
			}
			if count > 0 {
				if time.Since(startTime) > time.Second {
					logger.Printf("%s valid after: %d iterations, %s\n",
						partitionDevice, count,
						format.Duration(time.Since(startTime)))
				} else {
					logger.Debugf(0, "%s valid after: %d iterations, %s\n",
						partitionDevice, count,
						format.Duration(time.Since(startTime)))
				}
			}
			doDelete = false
			return loopDevice, nil
		}
		sleeper.Sleep()
	}
	return "", fmt.Errorf("timed out waiting for partition: %s",
		partitionDevice)
}
