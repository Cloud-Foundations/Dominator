package backoffdelay

import (
	"time"
)

func newExponential(minimumDelay, maximumDelay time.Duration,
	growthRate uint) *Exponential {
	if minimumDelay <= 0 {
		minimumDelay = time.Second
	}
	if maximumDelay <= minimumDelay {
		maximumDelay = 10 * minimumDelay
	}
	return &Exponential{
		growthRate: growthRate,
		interval:   minimumDelay,
		maximum:    maximumDelay,
		minimum:    minimumDelay,
		sleepFunc:  time.Sleep,
	}
}

func (e *Exponential) getAndUpdateInterval() time.Duration {
	retval := e.interval
	e.interval += e.interval >> e.growthRate
	if e.interval > e.maximum {
		e.interval = e.maximum
	}
	return retval
}

func (e *Exponential) reset() {
	e.interval = e.minimum
}

func (e *Exponential) remainingInterval() time.Duration {
	return time.Until(e.stopTime)
}

func (e *Exponential) sleep() {
	e.sleepFunc(e.getAndUpdateInterval())
}

func (e *Exponential) sleepUntilEnd() {
	e.sleepFunc(e.RemainingInterval())
}

func (e *Exponential) startInterval() {
	e.stopTime = time.Now().Add(e.getAndUpdateInterval())
}
