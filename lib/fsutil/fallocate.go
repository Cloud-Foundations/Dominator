package fsutil

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func fallocate(filename string, size uint64) error {
	fd, err := syscall.Open(filename, syscall.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	return wsyscall.Fallocate(int(fd), wsyscall.FALLOC_FL_KEEP_SIZE,
		0, int64(size))
}

func fallocateOrFill(filename string, size uint64,
	logger log.DebugLogger) error {
	err := Fallocate(filename, size)
	if err == nil {
		return nil
	}
	// Some kind of error trying to allocate. Check if big enough already.
	var statbuf wsyscall.Stat_t
	if err := wsyscall.Stat(filename, &statbuf); err != nil {
		return err
	}
	if uint64(statbuf.Blocks) >= size>>9 {
		return nil // Big enough already.
	}
	if !errors.Is(err, syscall.ENOTSUP) {
		return err
	}
	// File allocation is not supported, and file isn't big enough. Fill it.
	logger.Printf("unable to fallocate, writing zeros to: %s\n", filename)
	startTime := time.Now()
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	buffer := make([]byte, 1<<20)
	for bytesRemaining := uint64(size); bytesRemaining > 0; {
		bytesToWrite := bytesRemaining
		if bytesToWrite > uint64(len(buffer)) {
			bytesToWrite = uint64(len(buffer))
		}
		bytesWritten, err := file.Write(buffer[:bytesToWrite])
		if err != nil {
			return err
		}
		bytesRemaining -= uint64(bytesWritten)
	}
	if err := file.Close(); err != nil {
		return err
	}
	timeTaken := time.Since(startTime)
	logger.Debugf(0, "wrote %s of zeros to: %s in: %s (%s/s)\n",
		format.FormatBytes(size), filename,
		format.Duration(timeTaken),
		format.FormatBytes(uint64(float64(size)/timeTaken.Seconds())))
	return nil
}
