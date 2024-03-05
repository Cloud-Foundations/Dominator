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

func (e *Exponential) reset() {
	e.interval = e.minimum
}

func (e *Exponential) sleep() {
	e.sleepFunc(e.interval)
	e.interval += e.interval >> e.growthRate
	if e.interval > e.maximum {
		e.interval = e.maximum
	}
}
