package concurrentlimit

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
)

func newLimiter(limits Limits) *Limiter {
	copied := make(Limits, len(limits))
	for method, limit := range limits {
		copied[method] = limit
	}
	return &Limiter{
		counts: make(map[userMethodType]uint, len(copied)),
		limits: copied,
	}
}

func (l *Limiter) acquire(method, username string,
	bypass bool) (func(), error) {
	if bypass {
		return nil, nil
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	limit := l.limits[method]
	if limit < 1 {
		return nil, nil
	}
	key := userMethodType{method: method, username: username}
	if count := l.counts[key]; count >= limit {
		return nil, errors.NewResourceExhaustedError(method,
			"per_user_method_concurrency")
	}
	l.counts[key]++
	return func() { l.release(key) }, nil
}

func (l *Limiter) release(key userMethodType) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	count := l.counts[key]
	if count < 1 {
		panic(fmt.Sprintf(
			"concurrentlimit: no calls to release for user=%q method=%q",
			key.username, key.method))
	}
	if count--; count < 1 {
		delete(l.counts, key)
	} else {
		l.counts[key] = count
	}
}
