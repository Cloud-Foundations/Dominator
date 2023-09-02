package backoffdelay

import (
	"time"
)

type exponential struct {
	growthRate uint
	interval   time.Duration
	maximum    time.Duration
	minimum    time.Duration
	sleep      func(time.Duration)
}

func newExponential(minimumDelay, maximumDelay time.Duration,
	growthRate uint) *exponential {
	if minimumDelay <= 0 {
		minimumDelay = time.Second
	}
	if maximumDelay <= minimumDelay {
		maximumDelay = 10 * minimumDelay
	}
	return &exponential{
		growthRate: growthRate,
		interval:   minimumDelay,
		maximum:    maximumDelay,
		minimum:    minimumDelay,
		sleep:      time.Sleep,
	}
}

func (e *exponential) Sleep() {
	e.sleep(e.interval)
	e.interval += e.interval >> e.growthRate
	if e.interval > e.maximum {
		e.interval = e.maximum
	}
}
