package queue

import (
	"container/list"
	"sync"
)

type senderType[T any] struct {
	channel        chan<- T
	lengthRecorder LengthRecorder
	mutex          sync.Mutex // Protect everything below.
	closing        bool
	queue          *list.List
}

func newReceiveChannel[T any](size uint,
	lengthRecorder LengthRecorder) (*senderType[T], <-chan T) {
	channel := make(chan T, size)
	if lengthRecorder == nil {
		lengthRecorder = dummyLengthRecorder
	}
	return &senderType[T]{
		channel:        channel,
		lengthRecorder: lengthRecorder,
		queue:          list.New(),
	}, channel
}

func (s *senderType[T]) Close() {
	s.mutex.Lock()
	s.closing = true
	if s.queue.Len() < 1 {
		close(s.channel)
	}
	s.mutex.Unlock()
}

func (s *senderType[T]) Send(value T) {
	s.mutex.Lock()
	if s.closing {
		s.mutex.Unlock()
		panic("cannot Send() on a closed channel")
	}
	if s.queue.Len() > 0 {
		s.queue.PushBack(value)
		length := uint(s.queue.Len())
		s.mutex.Unlock()
		s.lengthRecorder(length)
		return
	}
	select {
	case s.channel <- value:
		s.mutex.Unlock()
		return
	default:
	}
	s.queue.PushBack(value)
	length := uint(s.queue.Len())
	s.mutex.Unlock()
	s.lengthRecorder(length)
	go s.processQueue()
}

// processQueue will dequeue entries and send to the channel. It returns when
// the queue empties.
func (s *senderType[T]) processQueue() {
	for {
		length := s.processQueueEntry()
		s.lengthRecorder(length)
		if length < 1 {
			return
		}
	}
}

// processQueueEntry will take one entry and send it to the channel and then
// remove the entry.
// It returns the number of entries in the queue.
func (s *senderType[T]) processQueueEntry() uint {
	s.mutex.Lock()
	entry := s.queue.Front()
	if entry == nil {
		s.mutex.Unlock()
		return 0
	}
	s.mutex.Unlock()
	s.channel <- entry.Value.(T)
	s.mutex.Lock()
	s.queue.Remove(entry)
	length := s.queue.Len()
	if s.closing && length < 1 {
		close(s.channel)
	}
	s.mutex.Unlock()
	return uint(length)
}
