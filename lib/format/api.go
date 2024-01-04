/*
Package format provides convenience functions for formatting.
*/
package format

var (
	TimeFormatSeconds    string = "02 Jan 2006 15:04:05 MST"
	TimeFormatSubseconds string = "02 Jan 2006 15:04:05.99 MST"
)

// FormatMilli formats the value in thousants as a floating point number.
func FormatMilli(value uint64) string {
	_, output := formatMilli(value)
	return output
}
