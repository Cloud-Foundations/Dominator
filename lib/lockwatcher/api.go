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
	CheckInterval      time.Duration // Default: 5 seconds, minimum: 1 second.
	Function           func()
	Logger             log.DebugLogger
	RFunction          func()
	LogTimeout         time.Duration // Default: 1 second, min: 1 millisecond.
	MaximumTryInterval time.Duration // Default/maximum: LogTimeout/32.
	MinimumTryInterval time.Duration // Default/maximum: LogTimeout/256.
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

func (lw *LockWatcher) GetOptions() LockWatcherOptions {
	return lw.LockWatcherOptions
}

func (lw *LockWatcher) GetStats() LockWatcherStats {
	return lw.getStats()
}

func (lw *LockWatcher) Stop() {
	lw.stop()
}

// WriteHtml will write HTML-formatted statistics information to writer, with an
// optional prefix. It returns true if something was written, else false.
func (lw *LockWatcher) WriteHtml(writer io.Writer,
	prefix string) (bool, error) {
	return lw.writeHtml(writer, prefix)
}
