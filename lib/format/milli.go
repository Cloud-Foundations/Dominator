package format

import (
	"fmt"
)

func formatMilli(value uint64) (string, string) {
	field := fmt.Sprintf("%.3f", float64(value)*1e-3)
	for pos := len(field) - 1; pos > 0; pos-- {
		if field[pos] == '.' {
			return field, field[:pos]
		}
		if field[pos] != '0' {
			return field, field[:pos+1]
		}
	}
	return field, field
}
