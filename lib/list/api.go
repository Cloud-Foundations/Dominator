package list

type List[T any] struct {
	first  *ListEntry[T]
	last   *ListEntry[T]
	length uint
}

type ListEntry[T any] struct {
	list     *List[T]
	next     *ListEntry[T]
	previous *ListEntry[T]
	value    T
}

// New creates a linked list of entries.
func New[T any]() *List[T] {
	return newList[T]()
}

// Back returns the last entry in the list if there is an entry, else nil.
func (l *List[T]) Back() *ListEntry[T] {
	return l.first
}

// Front returns the first entry in the list if there is an entry, else nil.
func (l *List[T]) Front() *ListEntry[T] {
	return l.first
}

// IterateEntries will call fn for each entry in the list, starting from the
// front. If fn returns false the iteration terminates and Iterate will return
// false, else it will return true.
// It is safe to remove the entry passed to fn.
func (l *List[T]) IterateEntries(fn func(*ListEntry[T]) bool) bool {
	return l.iterateEntries(fn)
}

// IterateValues will call fn for each entry in the list, starting from the
// front. If fn returns false the iteration terminates and IterateValues will
// return false, else it will return true.
// It is safe to remove the entry corresponding to the value passed to fn.
func (l *List[T]) IterateValues(fn func(T) bool) bool {
	return l.iterateValues(fn)
}

// Length returns the number of entries in the list.
func (l *List[T]) Length() uint {
	return l.length
}

// PushBack adds/moves the value to the back of the list. It returns the list
// entry.
func (l *List[T]) PushBack(value T) *ListEntry[T] {
	return l.pushBack(value)
}

// PushFont adds/moves the value to the front of the list. It returns the list
// entry.
func (l *List[T]) PushFront(value T) *ListEntry[T] {
	return l.pushFront(value)
}

// Next returns the next entry in the list after the specified entry if there
// is a next entry, else it returns nil.
func (e *ListEntry[T]) Next() *ListEntry[T] {
	return e.next
}

// Previous returns the previous entry in the list before the specified if there
// is a previous entry, else it returns nil.
func (e *ListEntry[T]) Previous() *ListEntry[T] {
	return e.previous
}

// Remove removes the entry from the list it belongs to.
func (e *ListEntry[T]) Remove() {
	e.remove()
}

// Value returns the value for the entry.
func (e *ListEntry[T]) Value() T {
	return e.value
}

type UniqueList[T comparable] struct {
	entries map[T]*UniqueListEntry[T]
	first   *UniqueListEntry[T]
	last    *UniqueListEntry[T]
}

type UniqueListEntry[T comparable] struct {
	list     *UniqueList[T]
	next     *UniqueListEntry[T]
	previous *UniqueListEntry[T]
	value    T
}

// NewUnique creates a linked list of unique entries.
func NewUnique[T comparable]() *UniqueList[T] {
	return newUniqueList[T]()
}

// Back returns the last entry in the list if there is an entry, else nil.
func (l *UniqueList[T]) Back() *UniqueListEntry[T] {
	return l.first
}

// Front returns the first entry in the list if there is an entry, else nil.
func (l *UniqueList[T]) Front() *UniqueListEntry[T] {
	return l.first
}

// Get returns the list entry for the specified value if present, else nil.
func (l *UniqueList[T]) Get(value T) *UniqueListEntry[T] {
	return l.entries[value]
}

// IterateEntries will call fn for each entry in the list, starting from the
// front. If fn returns false the iteration terminates and IterateEntries will
// return false, else it will return true.
// It is safe to remove the entry passed to fn.
func (l *UniqueList[T]) IterateEntries(fn func(*UniqueListEntry[T]) bool) bool {
	return l.iterateEntries(fn)
}

// IterateValues will call fn for each entry in the list, starting from the
// front. If fn returns false the iteration terminates and IterateValues will
// return false, else it will return true.
// It is safe to remove the entry corresponding to the value passed to fn.
func (l *UniqueList[T]) IterateValues(fn func(T) bool) bool {
	return l.iterateValues(fn)
}

// Length returns the number of entries in the list.
func (l *UniqueList[T]) Length() uint {
	return uint(len(l.entries))
}

// PushBack adds/moves the value to the back of the list. It returns the list
// entry.
func (l *UniqueList[T]) PushBack(value T) *UniqueListEntry[T] {
	return l.pushBack(value)
}

// PushFont adds/moves the value to the front of the list. It returns the list
// entry.
func (l *UniqueList[T]) PushFront(value T) *UniqueListEntry[T] {
	return l.pushFront(value)
}

// Remove removes the entry with the corresponding value from the list.
func (l *UniqueList[T]) Remove(value T) {
	l.remove(value)
}

// Next returns the next entry in the list after the specified entry if there
// is a next entry, else it returns nil.
func (e *UniqueListEntry[T]) Next() *UniqueListEntry[T] {
	return e.next
}

// Previous returns the previous entry in the list before the specified if there
// is a previous entry, else it returns nil.
func (e *UniqueListEntry[T]) Previous() *UniqueListEntry[T] {
	return e.previous
}

// Remove removes the entry from the list it belongs to.
func (e *UniqueListEntry[T]) Remove() {
	e.remove()
}

// Value returns the value for the entry.
func (e *UniqueListEntry[T]) Value() T {
	return e.value
}
