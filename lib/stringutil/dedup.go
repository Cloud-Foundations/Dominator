package stringutil

func deduplicateList(list []string, makeIfEmpty bool) (
	[]string, map[string]struct{}) {
	if len(list) < 1 && !makeIfEmpty {
		return list, nil
	}
	copiedEntries := make(map[string]struct{}, len(list))
	outputList := make([]string, 0, len(list))
	for _, entry := range list {
		if _, ok := copiedEntries[entry]; !ok {
			outputList = append(outputList, entry)
			copiedEntries[entry] = struct{}{}
		}
	}
	if len(outputList) == len(list) {
		return list, copiedEntries
	}
	return outputList, copiedEntries
}

func (d *StringDeduplicator) clear() {
	if d.lock {
		d.mutex.Lock()
		defer d.mutex.Unlock()
	}
	d.mapping = make(map[string]string)
	d.statistics = StringDuplicationStatistics{}
}

func (d *StringDeduplicator) deDuplicate(str string) string {
	if str == "" {
		return ""
	}
	if d.lock {
		d.mutex.Lock()
		defer d.mutex.Unlock()
	}
	if cached, ok := d.mapping[str]; ok {
		d.statistics.DuplicateBytes += uint64(len(str))
		d.statistics.DuplicateStrings++
		return cached
	} else {
		d.mapping[str] = str
		d.statistics.UniqueBytes += uint64(len(str))
		d.statistics.UniqueStrings++
		return str
	}
}

func (d *StringDeduplicator) deleteUnregistered() {
	if d.lock {
		d.mutex.Lock()
		defer d.mutex.Unlock()
	}
	for str := range d.mapping {
		if _, registered := d.registered[str]; !registered {
			delete(d.mapping, str)
		}
	}
	d.registered = nil
}

func (d *StringDeduplicator) getStatistics() StringDuplicationStatistics {
	if d.lock {
		d.mutex.Lock()
		defer d.mutex.Unlock()
	}
	return d.statistics
}

func (d *StringDeduplicator) register(str string) {
	if d.lock {
		d.mutex.Lock()
		defer d.mutex.Unlock()
	}
	if d.registered == nil {
		d.registered = make(map[string]struct{})
	}
	d.registered[str] = struct{}{}
}
