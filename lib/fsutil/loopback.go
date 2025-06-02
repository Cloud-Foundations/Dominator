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

func loopbackDelete(loopDevice string, grabLock bool) error {
	if grabLock {
		losetupMutex.Lock()
		defer losetupMutex.Unlock()
	}
	return exec.Command("losetup", "-d", loopDevice).Run()
}

func loopbackDeleteAndWaitForPartition(loopDevice, partition string,
	timeout time.Duration, logger log.DebugLogger) error {
	losetupMutex.Lock()
	defer losetupMutex.Unlock()
	if err := loopbackDelete(loopDevice, false); err != nil {
		return err
	}
	// Wait for partition device to disappear. Deleting it directly might not be
	// safe because there may be a pending dynamic device node deletion event.
	partitionDevice := loopDevice + partition
	sleeper := backoffdelay.NewExponential(time.Millisecond,
		100*time.Millisecond, 2)
	startTime := time.Now()
	stopTime := startTime.Add(timeout)
	for count := 0; time.Until(stopTime) >= 0; count++ {
		if _, err := os.Stat(partitionDevice); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		sleeper.Sleep()
	}
	if time.Since(startTime) > 15*time.Second {
		if err := os.Remove(partitionDevice); err != nil {
			logger.Printf("failed to forcibly delete partition: %s: %s\n",
				partitionDevice, err)
		} else {
			logger.Printf("forcibly deleted partition: %s\n", partitionDevice)
		}
	}
	return fmt.Errorf("timed out waiting for partition to delete: %s",
		partitionDevice)
}

func loopbackSetup(filename string, grabLock bool) (string, error) {
	if grabLock {
		losetupMutex.Lock()
		defer losetupMutex.Unlock()
	}
	cmd := exec.Command("losetup", "-fP", "--show", filename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func loopbackSetupAndWaitForPartition(filename, partition string,
	timeout time.Duration, logger log.DebugLogger) (string, error) {
	losetupMutex.Lock()
	defer losetupMutex.Unlock()
	loopDevice, err := loopbackSetup(filename, false)
	if err != nil {
		return "", err
	}
	doDelete := true
	defer func() {
		if doDelete {
			loopbackDelete(loopDevice, false)
		}
	}()
	// Probe for partition device because it might not be immediately available.
	partitionDevice := loopDevice + partition
	startTime := time.Now()
	numIterations, numOpened, err := WaitForBlockAvailable(partitionDevice,
		timeout)
	if err == nil {
		doDelete = false
		if numIterations > 0 {
			if time.Since(startTime) > time.Second {
				logger.Printf(
					"%s valid after: %d iterations, %d opens, %s\n",
					partitionDevice, numIterations, numOpened,
					format.Duration(time.Since(startTime)))
			} else {
				logger.Debugf(0,
					"%s valid after: %d iterations, %d opens, %s\n",
					partitionDevice, numIterations, numOpened,
					format.Duration(time.Since(startTime)))
			}
		}
		return loopDevice, nil
	}
	if numOpened > 0 {
		if time.Since(startTime) > 15*time.Second {
			if err := os.Remove(partitionDevice); err != nil {
				logger.Printf("failed to forcibly delete partition: %s: %s\n",
					partitionDevice, err)
			} else {
				logger.Printf("forcibly deleted partition: %s\n",
					partitionDevice)
			}
		}
	}
	return "", err
}
