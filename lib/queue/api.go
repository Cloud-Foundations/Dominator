package queue

type BroadcastEntry[T any] struct {
	Position uint64
	Value    T
}

type BroadcastQueue[T any] interface {
	// CloseSubscriber closes a subscriber channel.
	CloseSubscriber(<-chan BroadcastEntry[T])

	// IterateEntries will call fn for each entry in the queue, starting from
	// the front (oldest). If fn returns false the iteration terminates and
	// IterateEntries will return false, else it will return true.
	IterateEntries(fn func(BroadcastEntry[T]) bool) bool

	// IterateEntriesReverse will call fn for each entry in the queue, starting
	// from the back (newest). If fn returns false the iteration terminates and
	// IterateEntriesReverse will return false, else it will return true.
	IterateEntriesReverse(fn func(BroadcastEntry[T]) bool) bool

	// IterateValues will call fn for each entry in the queue, starting from
	// the front (oldest). If fn returns false the iteration terminates and
	// IterateValues will return false, else it will return true.
	IterateValues(fn func(T) bool) bool

	// IterateValuesReverse will call fn for each entry in the queue, starting
	// from the back (newest). If fn returns false the iteration terminates and
	// IterateEntriesReverse will return false, else it will return true.
	IterateValuesReverse(fn func(T) bool) bool

	// Position returns the position of the next entry that will be sent.
	Position() uint64

	// Subscribe creates a subscriber channel which will receive messages at the
	// specified startingPostion.
	Subscribe(startingPosition uint64) <-chan BroadcastEntry[T]

	// Sync ensures that any pending messages have been added to the queue.
	Sync()
}

type CloseSender[T any] interface {
	Closer
	Sender[T]
}

type Closer interface {
	Close()
}

type LengthRecorder func(uint)

type Queue[T any] interface {
	Receiver[T]
	Sender[T]
}

type Receiver[T any] interface {
	Receive() (T, bool)
	ReceiveAll() []T
}

type Sender[T any] interface {
	Send(value T)
}

// NewBroadcastQueue will create a one-to-many queue which may be used to
// broadcast messages. The starting position of the queue is given by
// initialPosition. The maximum number of entries that may be stored in the
// queue is given by maximumSize; zero will allow the queue to grow without
// bounds.
// A BroadcastQueue is returned which may be used to register subscribers
// (consumers), a channel to write messages and a channel yielding messages
// which have been removed (due to the queue filling up).
// A background goroutine is created for the queue and for each subscriber.
func NewBroadcastQueue[T any](initialPosition, maximumSize uint64) (
	BroadcastQueue[T], chan<- T, <-chan BroadcastEntry[T]) {
	return newBroadcastQueue[T](initialPosition, maximumSize)
}

// NewChannelPair creates a pair of channels (a send-only channel and a
// receive-only channel) which form a queue. Data of type T may be sent via the
// queue. The send-only channel is always available for sending. Data are stored
// in an internal buffer until they are dequeued by reading from the
// receive-only channel. If the send-only channel is closed the receive-only
// channel will be closed after all data are consumed.
// If lengthRecorder is not nil, it will be called to record the length of the
// queue whenever it changes.
// A background goroutine is created.
func NewChannelPair[T any](lengthRecorder LengthRecorder) (chan<- T, <-chan T) {
	return newChannelPair[T](lengthRecorder)
}

// NewDataQueue creates a pair of channels (a send-only channel and a
// receive-only channel) which form a queue. Arbitrary data may be sent via the
// queue. The send-only channel is always available for sending. Data are stored
// in an internal buffer until they are dequeued by reading from the
// receive-only channel. If the send-only channel is closed the receive-only
// channel will be closed after all data are consumed.
// A background goroutine is created.
func NewDataQueue() (chan<- interface{}, <-chan interface{}) {
	return newDataQueue()
}

// NewEventQueue creates a pair of channels (a send-only channel and a
// receive-only channel) which form a queue. Events (empty structures) may be
// sent via the queue. The send-only channel is always available for sending.
// An internal count of events received but not consumed is maintained. If the
// send-only channel is closed the receive-only channel will be closed after all
// events are consumed.
// A background goroutine is created.
func NewEventQueue() (chan<- struct{}, <-chan struct{}) {
	return newEventQueue()
}

// NewQueue creates a queue of the specified type.
// The Send method always succeeds and will store data in an internal buffer.
// The Receive method always succeeds and returns true if it received data.
// The ReceiveAll method always succeeds and returns a slice of T containing
// the data received.
// No goroutine is created.
// If lengthRecorder is not nil, it will be called to record the length of the
// internal buffer whenever it changes.
func NewQueue[T any](lengthRecorder LengthRecorder) Queue[T] {
	return newQueue[T](lengthRecorder)
}

// NewReceiveChannel creates a channel that may be used for reading and a
// corresponding CloseSender type which may be used to send data using the Send
// method. The channel size is specified by size.
// The Close method will close the channel after all data are received.
// The Send method always succeeds and will store data in an internal buffer if
// the channel is full.
// A background goroutine is created when the channel fills, and is destroyed
// when the internal buffer empties.
// If lengthRecorder is not nil, it will be called to record the length of the
// internal buffer whenever it changes.
func NewReceiveChannel[T any](size uint,
	lengthRecorder LengthRecorder) (CloseSender[T], <-chan T) {
	return newReceiveChannel[T](size, lengthRecorder)
}
