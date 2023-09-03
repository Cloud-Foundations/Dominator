package fsutil

import (
	"path"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/fsnotify/fsnotify"
)

var (
	lock     sync.RWMutex
	watchers []*fsnotify.Watcher
)

func watchFileWithFsNotify(pathname string, logger log.Logger) <-chan struct{} {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Println("Error creating watcher:", err)
		return nil
	}
	lock.Lock()
	defer lock.Unlock()
	watchers = append(watchers, watcher)
	pathname = path.Clean(pathname)
	dirname := path.Dir(pathname)
	if err := watcher.Add(dirname); err != nil {
		logger.Println("Error adding watch:", err)
		return nil
	}
	channel := make(chan struct{}, 1)
	go waitForNotifyEvents(watcher, pathname, channel, logger)
	return channel
}

func watchFileStopWithFsNotify() bool {
	lock.Lock()
	defer lock.Unlock()
	// Send cleanup notification to watchers.
	for _, watcher := range watchers {
		watcher.Close()
	}
	// Wait for cleanup of each watcher.
	for _, watcher := range watchers {
		for {
			if _, ok := <-watcher.Events; !ok {
				break
			}
		}
	}
	watchers = nil
	return true
}

func waitForNotifyEvents(watcher *fsnotify.Watcher, pathname string,
	channel chan<- struct{}, logger log.Logger) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if path.Clean(event.Name) != pathname {
				continue
			}
			channel <- struct{}{}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Println("Error with watcher:", err)
		}
	}
}
