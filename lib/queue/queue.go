package queue

import (
	"container/list"
	"sync"
)

type queueType[T any] struct {
	lengthRecorder LengthRecorder
	mutex          sync.Mutex // Protect everything below.
	queue          *list.List
}

func newQueue[T any](lengthRecorder LengthRecorder) *queueType[T] {
	if lengthRecorder == nil {
		lengthRecorder = dummyLengthRecorder
	}
	return &queueType[T]{
		lengthRecorder: lengthRecorder,
		queue:          list.New(),
	}
}

func (s *queueType[T]) Receive() (T, bool) {
	s.mutex.Lock()
	entry := s.queue.Front()
	if entry == nil {
		s.mutex.Unlock()
		var empty T
		return empty, false
	}
	s.queue.Remove(entry)
	length := uint(s.queue.Len())
	s.mutex.Unlock()
	s.lengthRecorder(length)
	return entry.Value.(T), true
}

func (s *queueType[T]) ReceiveAll() []T {
	var retval []T
	s.mutex.Lock()
	for {
		entry := s.queue.Front()
		if entry == nil {
			s.mutex.Unlock()
			s.lengthRecorder(0)
			return retval
		}
		retval = append(retval, entry.Value.(T))
		s.queue.Remove(entry)
	}
}

func (s *queueType[T]) Send(value T) {
	s.mutex.Lock()
	s.queue.PushBack(value)
	length := uint(s.queue.Len())
	s.mutex.Unlock()
	s.lengthRecorder(length)
}
