package lockwatcher

import (
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
)

func (lw *LockWatcher) loop(check func(), stopChannel <-chan struct{}) {
	for {
		timer := time.NewTimer(lw.CheckInterval)
		select {
		case <-stopChannel:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			check()
		}
	}
}

func newLockWatcher(lock sync.Locker, options LockWatcherOptions) *LockWatcher {
	if options.CheckInterval < time.Second {
		options.CheckInterval = 5 * time.Second
	}
	if options.LogTimeout < time.Millisecond {
		options.LogTimeout = time.Second
	}
	rstopChannel := make(chan struct{}, 1)
	stopChannel := make(chan struct{}, 1)
	lockWatcher := &LockWatcher{
		LockWatcherOptions: options,
		lock:               lock,
		rstopChannel:       rstopChannel,
		stopChannel:        stopChannel,
	}
	if _, ok := lock.(RWLock); ok {
		go lockWatcher.loop(lockWatcher.rcheck, rstopChannel)
		go lockWatcher.loop(lockWatcher.wcheck, stopChannel)
	} else {
		go lockWatcher.loop(lockWatcher.check, stopChannel)
	}
	return lockWatcher
}

func (lw *LockWatcher) check() {
	lockedChannel := make(chan struct{}, 1)
	timer := time.NewTimer(lw.LogTimeout)
	go func() {
		lw.lock.Lock()
		lockedChannel <- struct{}{}
		if lw.Function != nil {
			lw.Function()
		}
		lw.lock.Unlock()
	}()
	select {
	case <-lockedChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
	}
	lw.Logger.Println("timed out getting lock")
	<-lockedChannel
	lw.Logger.Println("eventually got lock")
}

func (lw *LockWatcher) rcheck() {
	lockedChannel := make(chan struct{}, 1)
	timer := time.NewTimer(lw.LogTimeout)
	rwlock := lw.lock.(RWLock)
	go func() {
		rwlock.RLock()
		lockedChannel <- struct{}{}
		if lw.RFunction != nil {
			lw.RFunction()
		}
		rwlock.RUnlock()
	}()
	select {
	case <-lockedChannel:
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
	}
	lw.Logger.Println("timed out getting rlock")
	<-lockedChannel
	lw.Logger.Println("eventually got rlock")
}

// rwcheck uses a TryLock() to grab a write lock, so as to not block future read
// lockers.
func (lw *LockWatcher) wcheck() {
	rwlock := lw.lock.(RWLock)
	timeoutTime := time.Now().Add(lw.LogTimeout)
	for ; time.Until(timeoutTime) > 0; time.Sleep(lw.LogTimeout >> 4) {
		if rwlock.TryLock() {
			if lw.Function != nil {
				lw.Function()
			}
			rwlock.Unlock()
			return
		}
	}
	lw.Logger.Println("timed out getting wlock")
	sleeper := backoffdelay.NewExponential(lw.LogTimeout>>4, time.Second, 1)
	for ; true; sleeper.Sleep() {
		if rwlock.TryLock() {
			if lw.Function != nil {
				lw.Function()
			}
			rwlock.Unlock()
			break
		}
	}
	lw.Logger.Println("eventually got wlock")
}

func (lw *LockWatcher) stop() {
	select {
	case lw.rstopChannel <- struct{}{}:
	default:
	}
	select {
	case lw.stopChannel <- struct{}{}:
	default:
	}
}
