package lockwatcher

import (
	"io"
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
	statsMutex   sync.RWMutex
	stats        LockWatcherStats
}

type LockWatcherOptions struct {
	CheckInterval time.Duration // Default: 5 seconds, minimum: 1 second.
	Function      func()
	Logger        log.DebugLogger
	RFunction     func()
	LogTimeout    time.Duration // Default: 1 second, minumum: 1 millisecond.
}

type LockWatcherStats struct {
	NumLockTimeouts  uint64 // Populated for sync.Mutex
	NumRLockTimeouts uint64 // Populated for sync.RWMutex
	NumWLockTimeouts uint64 // Populated for sync.RWMutex
	WaitingForLock   bool   // Populated for sync.Mutex
	WaitingForRLock  bool   // Populated for sync.RWMutex
	WaitingForWLock  bool   // Populated for sync.RWMutex
}

func New(lock sync.Locker, options LockWatcherOptions) *LockWatcher {
	return newLockWatcher(lock, options)
}

func (lw *LockWatcher) GetStats() LockWatcherStats {
	return lw.getStats()
}

func (lw *LockWatcher) Stop() {
	lw.stop()
}

func (lw *LockWatcher) WriteHtml(writer io.Writer, prefix string) {
	lw.writeHtml(writer, prefix)
}
