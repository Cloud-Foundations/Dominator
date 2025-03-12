package queue

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
