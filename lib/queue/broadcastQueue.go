package queue

import (
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/list"
)

// sendAndRecover will send a value to a channel. It returns true if the value
// was sent, else false if the channel was closed.
func sendAndRecover[T any](ch chan<- T, value T) (sent bool) {
	defer func() {
		recover()
	}()
	ch <- value
	return true
}

type broadcastQueue[T any] struct {
	freshMeat   *sync.Cond
	maximumSize uint64
	sync        chan<- struct{}
	mutex       sync.Mutex // Protect everything below.
	position    uint64
	queue       *list.List[BroadcastEntry[T]]
	subscribers map[<-chan BroadcastEntry[T]]chan<- BroadcastEntry[T]
}

func newBroadcastQueue[T any](initialPosition, maximumSize uint64) (
	BroadcastQueue[T], chan<- T, <-chan BroadcastEntry[T]) {
	syncChannel := make(chan struct{}) // Ensure exclusion from adding.
	q := &broadcastQueue[T]{
		sync:        syncChannel,
		maximumSize: maximumSize,
		position:    initialPosition,
		queue:       list.New[BroadcastEntry[T]](),
		subscribers: make(map[<-chan BroadcastEntry[T]]chan<- BroadcastEntry[T]),
	}
	q.freshMeat = sync.NewCond(&q.mutex)
	addChannel := make(chan T) // Ensure exclusion from syncing.
	var removedChannel chan BroadcastEntry[T]
	if maximumSize > 0 {
		removedChannel = make(chan BroadcastEntry[T], 1)
	}
	go q.manage(addChannel, removedChannel, syncChannel)
	return q, addChannel, removedChannel
}

func (q *broadcastQueue[T]) manage(addChannel <-chan T,
	removedChannel chan<- BroadcastEntry[T], syncChannel <-chan struct{}) {
	for {
		select {
		case newMessage := <-addChannel:
			q.mutex.Lock()
			q.queue.PushBack(BroadcastEntry[T]{
				Position: q.position,
				Value:    newMessage,
			})
			q.position++
			q.mutex.Unlock()
			q.freshMeat.Broadcast()
		case <-syncChannel:
			continue
		}
		if q.maximumSize > 0 && uint64(q.queue.Length()) > q.maximumSize {
			entry := q.queue.Front()
			q.mutex.Lock()
			entry.Remove()
			q.mutex.Unlock()
			removedChannel <- entry.Value()
		}
	}
}

func (q *broadcastQueue[T]) CloseSubscriber(ch <-chan BroadcastEntry[T]) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	close(q.subscribers[ch])
	delete(q.subscribers, ch)
}

func (q *broadcastQueue[T]) IterateEntries(
	fn func(BroadcastEntry[T]) bool) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.queue.IterateValues(fn)
}

func (q *broadcastQueue[T]) IterateEntriesReverse(
	fn func(BroadcastEntry[T]) bool) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.queue.IterateValuesReverse(fn)
}

func (q *broadcastQueue[T]) IterateValues(fn func(T) bool) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.queue.IterateValues(func(entry BroadcastEntry[T]) bool {
		return fn(entry.Value)
	})
}

func (q *broadcastQueue[T]) IterateValuesReverse(fn func(T) bool) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.queue.IterateValuesReverse(func(entry BroadcastEntry[T]) bool {
		return fn(entry.Value)
	})
}

func (q *broadcastQueue[T]) Position() uint64 {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.position
}

func (q *broadcastQueue[T]) Subscribe(start uint64) <-chan BroadcastEntry[T] {
	channel := make(chan BroadcastEntry[T], 1)
	q.mutex.Lock()
	q.subscribers[channel] = channel
	q.mutex.Unlock()
	go q.consume(channel, start)
	return channel
}

func (q *broadcastQueue[T]) Sync() {
	q.sync <- struct{}{}
}

func (q *broadcastQueue[T]) consume(ch chan<- BroadcastEntry[T],
	position uint64) {
	var nextEntry *list.ListEntry[BroadcastEntry[T]]
	// Wait for first entry in queue.
	q.mutex.Lock()
	for nextEntry == nil {
		nextEntry = q.queue.Front()
		if nextEntry != nil {
			break
		}
		q.freshMeat.Wait()
	}
	// Skip past entries which are before the position we care about, so long as
	// there is a next entry.
	for {
		e := nextEntry.Next()
		if e != nil && position > nextEntry.Value().Position {
			nextEntry = e
		} else {
			break
		}
	}
	q.mutex.Unlock()
	for {
		if pos := nextEntry.Value().Position; pos >= position {
			if !sendAndRecover(ch, nextEntry.Value()) {
				return
			}
			position = pos
		}
		q.mutex.Lock()
		for {
			if e := nextEntry.Next(); e != nil {
				nextEntry = e
				q.mutex.Unlock()
				break
			}
			q.freshMeat.Wait()
		}
	}
}
