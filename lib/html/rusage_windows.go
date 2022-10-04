package html

import (
	"time"
)

func getRusage() (time.Time, time.Time) {
	return time.Now(), time.Now()
}
