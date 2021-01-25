package filter

import (
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func (mf *MergeableFilter) exportFilter() *Filter {
	if mf.filterLines == nil {
		return nil // Sparse filter.
	}
	filterLines := stringutil.ConvertMapKeysToList(mf.filterLines, true)
	return &Filter{FilterLines: filterLines}
}

func (mf *MergeableFilter) merge(filter *Filter) {
	if filter == nil {
		return // Sparse filter.
	}
	if mf.filterLines == nil {
		mf.filterLines = make(map[string]struct{}, len(filter.FilterLines))
	}
	for _, filterLine := range filter.FilterLines {
		mf.filterLines[filterLine] = struct{}{}
	}
}
