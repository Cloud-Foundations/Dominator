package connpool

import (
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/resourcepool"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

var (
	lock sync.Mutex
	pool *resourcepool.Pool
)

func getConnectionLimit() uint {
	maxFD, _, err := wsyscall.GetFileDescriptorLimit()
	if err != nil {
		return 900
	}
	maxConnAttempts := maxFD - 50
	maxConnAttempts = (maxConnAttempts / 100)
	if maxConnAttempts < 1 {
		maxConnAttempts = 1
	} else {
		maxConnAttempts *= 100
	}
	return uint(maxConnAttempts)
}

func getResourcePool() *resourcepool.Pool {
	// Delay setting of internal limits to allow application code to increase
	// the limit on file descriptors first.
	if pool == nil {
		lock.Lock()
		if pool == nil {
			pool = resourcepool.New(getConnectionLimit(), "connections")
		}
		lock.Unlock()
	}
	return pool
}
