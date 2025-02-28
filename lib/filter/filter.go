package filter

import (
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/pathregexp"
)

func load(filename string) (*Filter, error) {
	lines, err := fsutil.LoadLines(filename)
	if err != nil {
		return nil, err
	}
	return New(lines)
}

func newFilter(filterLines []string) (*Filter, error) {
	var filter Filter
	filter.FilterLines = make([]string, 0)
	for _, line := range filterLines {
		if line != "" {
			filter.FilterLines = append(filter.FilterLines, line)
		}
	}
	if err := filter.compile(); err != nil {
		return nil, err
	}
	return &filter, nil
}

func read(reader io.Reader) (*Filter, error) {
	lines, err := fsutil.ReadLines(reader)
	if err != nil {
		return nil, err
	}
	return New(lines)
}

func (filter *Filter) compile() error {
	filter.matchers = make([]pathregexp.Regexp, len(filter.FilterLines))
	for index, reEntry := range filter.FilterLines {
		if reEntry == "!" {
			filter.invertMatches = true
			continue
		}
		var err error
		filter.matchers[index], err = pathregexp.Compile(reEntry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (filter *Filter) listUnoptimised() []string {
	if len(filter.matchers) != len(filter.FilterLines) {
		filter.compile()
	}
	var lines []string
	for index, line := range filter.FilterLines {
		if ok := pathregexp.IsOptimised(filter.matchers[index]); !ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func (filter *Filter) match(pathname string) bool {
	if len(filter.matchers) != len(filter.FilterLines) {
		filter.compile()
	}
	defaultRetval := false
	matchRetval := true
	if filter.invertMatches {
		defaultRetval = true
		matchRetval = false
	}
	for _, matcher := range filter.matchers {
		if matcher != nil && matcher.MatchString(pathname) {
			return matchRetval
		}
	}
	return defaultRetval
}

func (filter *Filter) registerStrings(registerFunc func(string)) {
	if filter != nil {
		for _, str := range filter.FilterLines {
			registerFunc(str)
		}
	}
}

func (filter *Filter) replaceStrings(replaceFunc func(string) string) {
	if filter != nil {
		for index, str := range filter.FilterLines {
			filter.FilterLines[index] = replaceFunc(str)
		}
	}
}

func (filter *Filter) write(writer io.Writer) error {
	for _, line := range filter.FilterLines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}
	return nil
}
