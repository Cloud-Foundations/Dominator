package text

import (
	"io"
)

// A ColumnCollector may be used to collect lines of text with variable numbers
// and widths of columns, which can then be written out with aligned columns.
// The zero value is ready to use.
type ColumnCollector struct {
	currentLine []string
	lines       [][]string
	widths      []uint
}

// AddField will add a field (column in a line) to the current line being
// collected.
func (cc *ColumnCollector) AddField(str string) error {
	return cc.addField(str)
}

// CompleteLine marks the end of the current line being collected.
func (cc *ColumnCollector) CompleteLine() error {
	return cc.completeLine()
}

// WriteLeftAligned will write all the collected lines to the specified writer,
// aligning the columns on the left side. All collected data are cleared on
// success.
func (cc *ColumnCollector) WriteLeftAligned(w io.Writer) error {
	return cc.writeAligned(w)
}
