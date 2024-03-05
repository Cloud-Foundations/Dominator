package retry

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
)

type Params struct {
	MaxRetries   uint64               // Default: unlimited.
	RetryTimeout time.Duration        // Default: unlimited.
	Sleeper      backoffdelay.Sleeper // Default: 100 milliseconds.
}

// Retry will run the specified function until it returns true or retry limits
// are exceeded. It returns an error if retry limits are exceeded.
func Retry(fn func() bool, params Params) error {
	return retry(fn, params)
}
