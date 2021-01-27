package stringutil

import "sync"

type StringDeduplicator struct {
	lock       bool
	mutex      sync.Mutex
	mapping    map[string]string
	statistics StringDuplicationStatistics
}

type StringDuplicationStatistics struct {
	DuplicateBytes   uint64
	DuplicateStrings uint64
	UniqueBytes      uint64
	UniqueStrings    uint64
}

// ConvertListToMap will convert list entries to map keys. If makeIfEmpty is
// true then a map is made for an empty list, otherwise nil is returned.
func ConvertListToMap(list []string, makeIfEmpty bool) map[string]struct{} {
	return convertListToMap(list, makeIfEmpty)
}

// ConvertMapKeysToList will return a list of map keys.
func ConvertMapKeysToList(mapData map[string]struct{}, doSort bool) []string {
	return convertMapKeysToList(mapData, doSort)
}

// NewStringDeduplicator will create a StringDeduplicator which may be used to
// eliminate duplicate string contents. It maintains an internal map of unique
// strings. If lock is true then each method call will take an exclusive lock.
func NewStringDeduplicator(lock bool) *StringDeduplicator {
	return &StringDeduplicator{lock: lock, mapping: make(map[string]string)}
}

// Clear will clear the internal map and statistics.
func (d *StringDeduplicator) Clear() {
	d.clear()
}

// DeDuplicate will return a string which has the same contents as str. This
// method should be called for every string in the application.
func (d *StringDeduplicator) DeDuplicate(str string) string {
	return d.deDuplicate(str)
}

// GetStatistics will return de-duplication statistics.
func (d *StringDeduplicator) GetStatistics() StringDuplicationStatistics {
	return d.getStatistics()
}
