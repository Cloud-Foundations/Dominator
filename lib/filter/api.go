package filter

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/pathregexp"
)

// A Filter contains a list of regular expressions matching pathnames which
// should be filtered out: excluded when building or not changed when pushing
// images to a sub.
// A Filter with no lines is an empty filter (nothing is excluded, everything is
// changed when pushing).
// A nil *Filter is a sparse filter: when building nothing is excluded. When
// pushing to a sub, all files are pushed but files on the sub which are not in
// the image are not removed from the sub.
type Filter struct {
	FilterLines   []string
	matchers      []pathregexp.Regexp
	invertMatches bool
}

// A MergeableFilter may be used to combine multiple Filters, eliminating
// duplicate match expressions.
type MergeableFilter struct {
	filterLines map[string]struct{}
}

// Load will load a Filter from a file containing newline separated regular
// expressions.
func Load(filename string) (*Filter, error) {
	return load(filename)
}

// New will create a Filter from a list of regular expressions, which are
// automatically anchored to the beginning of the string to be matched against.
// If filterLines is of length zero the Filter is an empty Filter.
func New(filterLines []string) (*Filter, error) {
	return newFilter(filterLines)
}

// Read will read a Filter from a reader containing newline separated regular
// expressions.
func Read(reader io.Reader) (*Filter, error) {
	return read(reader)
}

// Compile will compile the regular expression strings for later use.
func (filter *Filter) Compile() error {
	return filter.compile()
}

// Equal will return true if two filters contain the same filter lines.
func (left *Filter) Equal(right *Filter) bool {
	return left.equal(right)
}

// ListUnoptimised returns the unoptimised regular expressions.
func (filter *Filter) ListUnoptimised() []string {
	return filter.listUnoptimised()
}

// Match will return true if pathname matches one of the regular expressions.
// The Compile method will be automatically called if it has not been called
// yet.
func (filter *Filter) Match(pathname string) bool {
	return filter.match(pathname)
}

// RegisterStrings may be used to register the regular expression strings with
// a string de-duper. This can be used for garbage collection.
func (filter *Filter) RegisterStrings(registerFunc func(string)) {
	filter.registerStrings(registerFunc)
}

// ReplaceStrings may be used to replace the regular expression strings with
// de-duplicated copies.
func (filter *Filter) ReplaceStrings(replaceFunc func(string) string) {
	filter.replaceStrings(replaceFunc)
}

// Write will write the filter as newline separated regular expressions.
func (filter *Filter) Write(writer io.Writer) error {
	return filter.write(writer)
}

// WriteHtml will write the filter with appropriate HTML markups.
func (filter *Filter) WriteHtml(writer io.Writer) {
	filter.writeHtml(writer)
}

// ExportFilter will return a Filter from previously merged Filters.
func (mf *MergeableFilter) ExportFilter() *Filter {
	return mf.exportFilter()
}

// Merge will merge a Filter.
func (mf *MergeableFilter) Merge(filter *Filter) {
	mf.merge(filter)
}
