/*
Package pathregexp implements regular expression search with pathname
optimisations.
*/
package pathregexp

type Regexp interface {
	MatchString(s string) bool
}

// Compile parses a regular expression and returns, if successful, a Regexp
// object that can be used to match against text. The expression is implictly
// anchored at the start of each string to match.
func Compile(expr string) (Regexp, error) {
	return compile(expr)
}

// IsOptimised will return true if a Regexp object is optimised for performance.
func IsOptimised(regex Regexp) bool {
	return isOptimised(regex)
}
