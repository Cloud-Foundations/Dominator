package lockwatcher

import (
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

type RWLock interface {
	sync.Locker
	RLock()
	RUnlock()
	TryLock() bool
}

type LockWatcher struct {
	lock sync.Locker
	LockWatcherOptions
	rstopChannel chan<- struct{}
	stopChannel  chan<- struct{}
}

type LockWatcherOptions struct {
	CheckInterval time.Duration // Default: 5 seconds, minimum: 1 second.
	Function      func()
	Logger        log.DebugLogger
	RFunction     func()
	LogTimeout    time.Duration // Default: 1 second, minumum: 1 millisecond.
}

func New(lock sync.Locker, options LockWatcherOptions) *LockWatcher {
	return newLockWatcher(lock, options)
}

func (lw *LockWatcher) Stop() {
	lw.stop()
}
