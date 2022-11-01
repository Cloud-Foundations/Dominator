package fsutil

import (
	"io"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

var stopped bool

func watchFile(pathname string, logger log.Logger) <-chan io.ReadCloser {
	channel := make(chan io.ReadCloser, 1)
	go watchFileForever(pathname, channel, logger)
	return channel
}

func watchFileForever(pathname string, channel chan<- io.ReadCloser,
	logger log.Logger) {
	var lastModTime time.Time
	for ; !stopped; time.Sleep(time.Second) {
		fileInfo, err := os.Stat(pathname)
		if err != nil {
			if logger != nil {
				logger.Printf("Error stating file: %s: %s\n", pathname, err)
			}
			continue
		}
		if fileInfo.ModTime() != lastModTime {
			if file, err := os.Open(pathname); err != nil {
				if logger != nil {
					logger.Printf("Error opening file: %s: %s\n", pathname, err)
				}
				continue
			} else {
				channel <- file
				lastModTime = fileInfo.ModTime()
			}
		}
	}
	close(channel)
}

func watchFileStop() {
	stopped = true
}
