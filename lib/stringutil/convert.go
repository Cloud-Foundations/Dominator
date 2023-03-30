package stringutil

import (
	"sort"
)

func convertListToMap(list []string, makeIfEmpty bool) map[string]struct{} {
	if len(list) < 1 && !makeIfEmpty {
		return nil
	}
	retval := make(map[string]struct{}, len(list))
	for _, entry := range list {
		retval[entry] = struct{}{}
	}
	return retval
}

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
