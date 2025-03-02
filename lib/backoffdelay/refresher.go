package backoffdelay

import (
	"time"
)

func newRefresher(deadline time.Time,
	minimumDelay, maximumDelay time.Duration) *Refresher {
	if minimumDelay <= 0 {
		minimumDelay = time.Second
	}
	return &Refresher{
		deadline:  deadline,
		maximum:   maximumDelay,
		minimum:   minimumDelay,
		sleepFunc: time.Sleep,
	}
}

func (r *Refresher) resetTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(r.WaitInterval())
}

func (r *Refresher) setDeadline(deadline time.Time) {
	r.deadline = deadline
}

func (r *Refresher) sleep() {
	r.sleepFunc(r.WaitInterval())
}

func (r *Refresher) waitInterval() time.Duration {
	interval := time.Until(r.deadline) >> 1
	if interval < r.minimum {
		interval = r.minimum
	}
	if r.maximum > 0 && interval > r.maximum {
		interval = r.maximum
	}
	return interval
}
