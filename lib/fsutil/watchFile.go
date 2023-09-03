//go:build !windows
// +build !windows

package fsutil

import (
	"io"
	"os"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

var stopChannel = make(chan struct{})

func watchFile(pathname string, logger log.Logger) <-chan io.ReadCloser {
	readCloserChannel := make(chan io.ReadCloser, 1)
	notifyChannel := watchFileWithFsNotify(pathname, logger)
	go watchFileForever(pathname, readCloserChannel, notifyChannel, logger)
	return readCloserChannel
}

func watchFileStop() {
	watchFileStopWithFsNotify()
	select {
	case stopChannel <- struct{}{}:
	default:
	}
}

func watchFileForever(pathname string, readCloserChannel chan<- io.ReadCloser,
	notifyChannel <-chan struct{}, logger log.Logger) {
	interval := time.Second
	if notifyChannel != nil {
		interval = 15 * time.Second
	}
	intervalTimer := time.NewTimer(0)
	var lastStat syscall.Stat_t
	lastFd := -1
	for {
		select {
		case <-intervalTimer.C:
		case <-notifyChannel:
			if !intervalTimer.Stop() {
				<-intervalTimer.C
			}
			if lastFd >= 0 {
				syscall.Close(lastFd)
			}
			lastFd = -1
		case <-stopChannel:
			if lastFd >= 0 {
				syscall.Close(lastFd)
			}
			close(readCloserChannel)
			return
		}
		intervalTimer = time.NewTimer(interval)
		var stat syscall.Stat_t
		if err := syscall.Stat(pathname, &stat); err != nil {
			if logger != nil {
				logger.Printf("Error stating file: %s: %s\n", pathname, err)
			}
			continue
		}
		if stat.Ino != lastStat.Ino {
			if file, err := os.Open(pathname); err != nil {
				if logger != nil {
					logger.Printf("Error opening file: %s: %s\n", pathname, err)
				}
				continue
			} else {
				// By holding onto the file, we guarantee that the inode number
				// for the file we've opened cannot be reused until we've seen
				// a new inode.
				if lastFd >= 0 {
					syscall.Close(lastFd)
				}
				lastFd, _ = wsyscall.Dup(int(file.Fd()))
				readCloserChannel <- file // Must happen after FD is duplicated.
				lastStat = stat
			}
		}
	}
}
