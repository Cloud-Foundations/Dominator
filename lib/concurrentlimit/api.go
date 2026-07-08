/*
Package concurrentlimit implements a per-(user, method) concurrency limiter
intended to be shared across SRPC, gRPC and REST handlers in a single process
so that concurrency caps cannot be bypassed by switching protocols.

Where lib/ratelimit caps admitted requests per unit time (throughput), this
package caps the number of simultaneously in-flight calls per identified
user for a given method (concurrency). The two are complementary: in a
typical server both run alongside each other, with the concurrency limiter
holding a slot for the duration of the handler and the rate limiter gating
admission at call start.

The Limiter exposes a protocol-neutral Acquire method. The SRPC adapter in
lib/srpc/serverutil (PerUserMethodLimiter) forwards to it via
srpc.MethodBlocker; future gRPC and REST adapters will call Acquire directly.
*/
package concurrentlimit

import (
	"sync"
)

// Limits maps a method name to the maximum number of concurrent calls
// permitted per user for that method. Methods absent from the map, and
// methods with a zero limit, are unlimited.
type Limits map[string]uint

// Limiter is a per-(user, method) concurrency limiter. Methods are safe for
// concurrent use.
type Limiter struct {
	mutex  sync.Mutex
	counts map[userMethodType]uint
	limits Limits
}

type userMethodType struct {
	method   string
	username string
}

// NewLimiter constructs a Limiter with the given per-method limits. The
// supplied map is copied; later mutations by the caller do not affect the
// Limiter.
func NewLimiter(limits Limits) *Limiter {
	return newLimiter(limits)
}

// Acquire reserves one concurrent-call slot for (method, username). It
// returns:
//   - (nil, nil) if bypass is true (e.g. the caller has elevated
//     method-power access), or if no limit is configured for the method;
//     no reservation is made in either case.
//   - (release, nil) on admit; the caller must invoke release exactly once
//     when the call completes to free the slot.
//   - (nil, *errors.ResourceExhaustedError) on denial when the user is
//     already at the limit for this method.
func (l *Limiter) Acquire(method, username string,
	bypass bool) (func(), error) {
	return l.acquire(method, username, bypass)
}
