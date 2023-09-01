package filter

import (
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func (left *Filter) equal(right *Filter) bool {
	if left == right {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	if len(left.FilterLines) != len(right.FilterLines) {
		return false
	}
	rightFilterLines := stringutil.ConvertListToMap(right.FilterLines, false)
	for _, leftFilterLine := range left.FilterLines {
		if _, ok := rightFilterLines[leftFilterLine]; !ok {
			return false
		}
	}
	return true
}
