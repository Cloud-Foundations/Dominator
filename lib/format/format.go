package format

import (
	"fmt"
	"time"
)

// Duration is similar to the time.Duration.String method from the standard
// library but is more readable and shows only 3 digits of precision when
// duration is less than 1 minute.
func Duration(duration time.Duration) string {
	if ns := duration.Nanoseconds(); ns < 1000 {
		return fmt.Sprintf("%dns", ns)
	} else if us := float64(duration) / float64(time.Microsecond); us < 1000 {
		return fmt.Sprintf("%.3gµs", us)
	} else if ms := float64(duration) / float64(time.Millisecond); ms < 1000 {
		return fmt.Sprintf("%.3gms", ms)
	} else if s := float64(duration) / float64(time.Second); s < 60 {
		return fmt.Sprintf("%.3gs", s)
	} else {
		duration -= duration % time.Second
		day := time.Hour * 24
		if duration < day {
			return duration.String()
		}
		week := day * 7
		if duration < week {
			days := duration / day
			duration %= day
			return fmt.Sprintf("%dd%s", days, duration)
		}
		year := day*365 + day>>2
		if duration < year {
			weeks := duration / week
			duration %= week
			days := duration / day
			duration %= day
			return fmt.Sprintf("%dw%dd%s", weeks, days, duration)
		}
		years := duration / year
		duration %= year
		weeks := duration / week
		duration %= week
		days := duration / day
		duration %= day
		return fmt.Sprintf("%dy%dw%dd%s", years, weeks, days, duration)
	}
}

// FormatBytes returns a string with the number of bytes specified converted
// into a human-friendly format with a binary multiplier (i.e. GiB).
func FormatBytes(bytes uint64) string {
	shift, multiplier := GetMiltiplier(bytes)
	return fmt.Sprintf("%d %sB", bytes>>shift, multiplier)
}

// GetMiltiplier will return the preferred base-2 multiplier (i.e. Ki, Mi, Gi)
// and right shift number for the specified vlaue.
func GetMiltiplier(value uint64) (uint, string) {
	if value>>40 > 100 || (value>>40 >= 1 && value&(1<<40-1) == 0) {
		return 40, "Ti"
	} else if value>>30 > 100 || (value>>30 >= 1 && value&(1<<30-1) == 0) {
		return 30, "Gi"
	} else if value>>20 > 100 || (value>>20 >= 1 && value&(1<<20-1) == 0) {
		return 20, "Mi"
	} else if value>>10 > 100 || (value>>10 >= 1 && value&(1<<10-1) == 0) {
		return 10, "Ki"
	} else {
		return 0, ""
	}
}
