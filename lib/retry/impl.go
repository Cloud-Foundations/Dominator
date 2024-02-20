package retry

import (
	"errors"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
)

var defaultSleeper = &simpleSleeper{}

type simpleSleeper struct{}

func retry(fn func() bool, params Params) error {
	params.prepare()
	stopTime := time.Now().Add(params.RetryTimeout)
	var tryCount uint64
	for {
		tryCount++
		if fn() {
			return nil
		}
		if params.RetryTimeout > 0 && time.Since(stopTime) >= 0 {
			return errors.New("timed out")
		}
		if params.MaxRetries > 0 && tryCount >= params.MaxRetries {
			return errors.New("too many retries")
		}
		params.Sleeper.Sleep()
	}
}

func (p *Params) prepare() {
	if p.Sleeper == nil {
		p.Sleeper = defaultSleeper
	} else if resetter, ok := p.Sleeper.(backoffdelay.Resetter); ok {
		resetter.Reset()
	}
}

func (s *simpleSleeper) Sleep() {
	time.Sleep(100 * time.Millisecond)
}
