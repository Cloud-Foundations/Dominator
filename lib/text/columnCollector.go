package text

import (
	"io"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func (cc *ColumnCollector) addField(str string) error {
	fieldWidth := uint(len(str))
	if index := len(cc.currentLine); index < len(cc.widths) {
		if fieldWidth > cc.widths[index] {
			cc.widths[index] = fieldWidth
		}
	} else {
		cc.widths = append(cc.widths, fieldWidth)
	}
	cc.currentLine = append(cc.currentLine, str)
	return nil
}

func (cc *ColumnCollector) completeLine() error {
	if len(cc.currentLine) < 1 {
		return nil
	}
	cc.lines = append(cc.lines, cc.currentLine)
	cc.currentLine = nil
	return nil
}

func (cc *ColumnCollector) writeAligned(w io.Writer) error {
	for _, line := range cc.lines {
		for index, field := range line {
			if _, err := w.Write([]byte(field)); err != nil {
				return err
			}
			if index+1 >= len(line) {
				if _, err := w.Write(newline); err != nil {
					return err
				}
				continue
			}
			numSpaces := cc.widths[index] + 1 - uint(len(field))
			for ; numSpaces > 0; numSpaces-- {
				if _, err := w.Write(space); err != nil {
					return err
				}
			}
		}
	}
	*cc = ColumnCollector{}
	return nil
}
