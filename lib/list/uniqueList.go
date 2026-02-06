package list

func newUniqueList[T comparable]() *UniqueList[T] {
	return &UniqueList[T]{
		entries: make(map[T]*UniqueListEntry[T]),
	}
}

func (l *UniqueList[T]) get(value T) *UniqueListEntry[T] {
	entry := l.entries[value]
	return entry
}

func (l *UniqueList[T]) iterateEntries(fn func(*UniqueListEntry[T]) bool) bool {
	var nextEntry *UniqueListEntry[T]
	for entry := l.first; entry != nil; entry = nextEntry {
		nextEntry = entry.next
		if !fn(entry) {
			return false
		}
	}
	return true
}

func (l *UniqueList[T]) iterateValues(fn func(T) bool) bool {
	return l.IterateEntries(func(entry *UniqueListEntry[T]) bool {
		return fn(entry.value)
	})
}

func (l *UniqueList[T]) pushBack(value T) *UniqueListEntry[T] {
	l.Remove(value)
	entry := &UniqueListEntry[T]{
		list:     l,
		previous: l.last,
		value:    value,
	}
	if l.last == nil {
		l.first = entry
	} else {
		l.last.next = entry
	}
	l.last = entry
	l.entries[value] = entry
	return entry
}

func (l *UniqueList[T]) pushFront(value T) *UniqueListEntry[T] {
	l.Remove(value)
	entry := &UniqueListEntry[T]{
		list:  l,
		next:  l.first,
		value: value,
	}
	if l.first == nil {
		l.last = entry
	} else {
		l.first.previous = entry
	}
	l.first = entry
	l.entries[value] = entry
	return entry
}

func (l *UniqueList[T]) remove(value T) {
	entry, ok := l.entries[value]
	if !ok {
		return
	}
	entry.Remove()
}

func (entry *UniqueListEntry[T]) remove() {
	delete(entry.list.entries, entry.value)
	if entry.previous == nil {
		entry.list.first = entry.next
	} else {
		entry.previous.next = entry.next
		entry.previous = nil
	}
	if entry.next == nil {
		entry.list.last = entry.previous
	} else {
		entry.next.previous = entry.previous
		entry.next = nil
	}
	entry.list = nil
}
