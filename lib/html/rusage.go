//go:build !windows

package html

import (
	"syscall"
	"time"
)

func getRusage() (time.Time, time.Time) {
	var rusage syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &rusage)
	return time.Unix(int64(rusage.Utime.Sec), int64(rusage.Utime.Usec)*1000),
		time.Unix(int64(rusage.Stime.Sec), int64(rusage.Stime.Usec)*1000)
}
