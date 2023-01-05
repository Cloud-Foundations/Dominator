package backoffdelay

import (
	"time"
)

type Sleeper interface {
	Sleep()
}

// NewExponential creates a Sleeper with specified minimum and maximum delays.
// If minimumDelay is less than or equal to 0, the default is 1 second.
// If maximumDelay is less than or equal to minimumDelay, the default is 10
// times minimumDelay.
// The Sleep interval will increase by a factor of 2 raised to the power of
// -growthRate. For example:
// 0: 1x
// 1: 0.5x
// 2: 0.25x
func NewExponential(minimumDelay, maximumDelay time.Duration,
	growthRate uint) Sleeper {
	return newExponential(minimumDelay, maximumDelay, growthRate)
}
