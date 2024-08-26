package backoffdelay

import (
	"time"
)

type Exponential struct {
	growthRate uint
	interval   time.Duration
	maximum    time.Duration
	minimum    time.Duration
	sleepFunc  func(time.Duration)
	stopTime   time.Time
}

type Resetter interface {
	Reset()
}

type Sleeper interface {
	Sleep()
}

// NewExponential creates a Sleeper with specified minimum and maximum delays.
// If minimumDelay is less than or equal to 0, the default is 1 second.
// If maximumDelay is less than or equal to minimumDelay, the default is 10
// times minimumDelay.
// The Sleep duration will increase by a factor of 2 raised to the power of
// -growthRate. For example:
// 0: 1x
// 1: 0.5x
// 2: 0.25x
func NewExponential(minimumDelay, maximumDelay time.Duration,
	growthRate uint) *Exponential {
	return newExponential(minimumDelay, maximumDelay, growthRate)
}

// RemainingInterval will return the time remaining until the end of the
// interval that was started when StartInterval was called.
func (e *Exponential) RemainingInterval() time.Duration {
	return e.remainingInterval()
}

// Reset will set the sleep duration to the minimum delay.
func (e *Exponential) Reset() {
	e.reset()
}

// Sleep will sleep and then increase the duration for the next Sleep, until
// reaching the maximum delay.
func (e *Exponential) Sleep() {
	e.sleep()
}

// SleepUntilEnd will sleep until the end of the interval that was started when
// StartInterval was called.
func (e *Exponential) SleepUntilEnd() {
	e.sleepUntilEnd()
}

// StartInterval starts an interval. This may be used to impose a maximum loop
// duration when iterating over a set of operations with individual timeouts.
func (e *Exponential) StartInterval() {
	e.startInterval()
}
