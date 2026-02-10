package queue

import (
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/list"
)

type queueType[T any] struct {
	lengthRecorder LengthRecorder
	mutex          sync.Mutex // Protect everything below.
	queue          *list.List[T]
}

func newQueue[T any](lengthRecorder LengthRecorder) *queueType[T] {
	if lengthRecorder == nil {
		lengthRecorder = dummyLengthRecorder
	}
	return &queueType[T]{
		lengthRecorder: lengthRecorder,
		queue:          list.New[T](),
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
	entry.Remove()
	length := uint(s.queue.Length())
	s.mutex.Unlock()
	s.lengthRecorder(length)
	return entry.Value(), true
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
		retval = append(retval, entry.Value())
		entry.Remove()
	}
}

func (s *queueType[T]) Send(value T) {
	s.mutex.Lock()
	s.queue.PushBack(value)
	length := uint(s.queue.Length())
	s.mutex.Unlock()
	s.lengthRecorder(length)
}
