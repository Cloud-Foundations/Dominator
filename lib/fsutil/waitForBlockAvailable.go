package fsutil

import (
	"fmt"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
)

func waitForBlockAvailable(pathname string,
	timeout time.Duration) (uint, uint, error) {
	if timeout < 0 || timeout > time.Hour {
		timeout = time.Hour
	}
	sleeper := backoffdelay.NewExponential(time.Millisecond,
		100*time.Millisecond, 2)
	startTime := time.Now()
	stopTime := startTime.Add(timeout)
	var numIterations, numOpened uint
	for ; time.Until(stopTime) >= 0; numIterations++ {
		// Need to open rather than just test for inode existance, because an
		// Open(2) is what may be needed to trigger dynamic device node creation
		if file, err := os.Open(pathname); err == nil {
			numOpened++
			fi, err := file.Stat()
			file.Close()
			if err != nil {
				return numIterations, numOpened, err
			}
			if fi.Mode()&os.ModeDevice != 0 {
				return numIterations, numOpened, nil
			}
		}
		sleeper.Sleep()
	}
	return numIterations, numOpened,
		fmt.Errorf("timed out waiting for partition, %d opens: %s",
			numOpened, pathname)
}
