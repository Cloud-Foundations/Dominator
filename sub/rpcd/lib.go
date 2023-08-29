package rpcd

import (
	"errors"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

const (
	maximumClientLockDuration = 15 * time.Second
)

// Check if another client has the client lock. The object lock must be
// held. Returns an error if another client has the lock.
func (t *rpcType) checkIfLockedByAnotherClient(conn *srpc.Conn) error {
	if t.lockedBy == nil {
		if !t.lockedUntil.IsZero() {
			t.lockedUntil = time.Time{}
		}
		return nil
	}
	if time.Since(t.lockedUntil) >= 0 {
		t.lockedBy = nil
		t.lockedUntil = time.Time{}
		return nil
	}
	if t.lockedBy == conn {
		return nil
	}
	return errors.New("another client has the lock")
}

// Try to grab the client lock. The object lock must be held. If duration is
// zero or less, only a check is performed.
// Returns an error if another client has the lock.
func (t *rpcType) getClientLock(conn *srpc.Conn, duration time.Duration) error {
	if err := t.checkIfLockedByAnotherClient(conn); err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}
	t.lockedBy = conn
	if duration > maximumClientLockDuration {
		duration = maximumClientLockDuration
	}
	t.lockedUntil = time.Now().Add(duration)
	return nil
}
