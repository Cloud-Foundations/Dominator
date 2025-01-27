package queue

type LengthRecorder func(uint)

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
