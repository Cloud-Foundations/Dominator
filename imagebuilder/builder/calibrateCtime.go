package builder

import (
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"time"
)

var (
	calibrateOnce    sync.Once
	calibrationError error
	_ctimeResolution time.Duration
)

// calibrateCtime will calibrate the resolution of inode change times for
// temporary files. It will return the minimum resolution or an error.
func calibrateCtime() (time.Duration, error) {
	buffer := []byte("data\n")
	file, err := ioutil.TempFile("", "ctimeCalibration")
	if err != nil {
		return 0, err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	fd := int(file.Fd()) // Bypass os.File overheads.
	if _, err := syscall.Write(fd, buffer); err != nil {
		return 0, err
	}
	var firstStat syscall.Stat_t
	if err := syscall.Stat(file.Name(), &firstStat); err != nil {
		return 0, err
	}
	interval := time.Nanosecond
	startTime := time.Now()
	for ; time.Since(startTime) < time.Second; interval *= 10 {
		if _, err := syscall.Write(fd, buffer); err != nil {
			return 0, err
		}
		var newStat syscall.Stat_t
		if err := syscall.Stat(file.Name(), &newStat); err != nil {
			return 0, err
		}
		if newStat.Ctim != firstStat.Ctim {
			return time.Since(startTime), nil
		}
		time.Sleep(interval)
	}
	return 0, errors.New("timed out calibrating Ctime changes")
}

func getCtimeResolution() (time.Duration, error) {
	calibrateOnce.Do(func() {
		_ctimeResolution, calibrationError = calibrateCtime()
	})
	return _ctimeResolution, calibrationError
}
