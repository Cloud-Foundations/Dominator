package formatter

// Pairs will format an arbitrary number of format,value pairs. If the value for
// a pair is the empty string, the pair is not included. This is convenient way
// to avoid formatting for empty strings. A trailing singleton is also included.
// The formatted string is returned.
func Pairs(pairs ...string) string {
	return formatPairs(pairs)
}
