package stringutil

import (
	"sort"
)

func convertMapKeysToList(mapData map[string]struct{}, doSort bool) []string {
	keys := make([]string, 0, len(mapData))
	for key := range mapData {
		keys = append(keys, key)
	}
	if doSort {
		sort.Strings(keys)
	}
	return keys
}
