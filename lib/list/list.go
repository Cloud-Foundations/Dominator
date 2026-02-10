package list

func newList[T any]() *List[T] {
	return &List[T]{}
}

func (l *List[T]) iterateEntries(fn func(*ListEntry[T]) bool) bool {
	var nextEntry *ListEntry[T]
	for entry := l.first; entry != nil; entry = nextEntry {
		nextEntry = entry.next
		if !fn(entry) {
			return false
		}
	}
	return true
}

func (l *List[T]) iterateValues(fn func(T) bool) bool {
	return l.IterateEntries(func(entry *ListEntry[T]) bool {
		return fn(entry.value)
	})
}

func (l *List[T]) pushBack(value T) *ListEntry[T] {
	entry := &ListEntry[T]{
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
	l.length++
	return entry
}

func (l *List[T]) pushFront(value T) *ListEntry[T] {
	entry := &ListEntry[T]{
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
	l.length++
	return entry
}

func (entry *ListEntry[T]) remove() {
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
	entry.list.length--
	entry.list = nil
}
