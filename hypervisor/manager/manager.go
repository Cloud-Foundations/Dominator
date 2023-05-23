package manager

import (
	"fmt"
	"time"
)

func (m *Manager) holdLock(timeout time.Duration, writeLock bool) error {
	if timeout > time.Minute {
		return fmt.Errorf("timeout: %s exceeds one minute", timeout)
	}
	if writeLock {
		m.mutex.Lock()
		time.Sleep(timeout)
		m.mutex.Unlock()
	} else {
		m.mutex.RLock()
		time.Sleep(timeout)
		m.mutex.RUnlock()
	}
	return nil
}
