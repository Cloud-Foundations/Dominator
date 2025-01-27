package queue

import "container/list"

func dummyLengthRecorder(length uint) {}

func newChannelPair[T any](lengthRecorder LengthRecorder) (chan<- T, <-chan T) {
	send := make(chan T, 1)
	receive := make(chan T, 1)
	if lengthRecorder == nil {
		lengthRecorder = dummyLengthRecorder
	}
	go manageQueue(send, receive, lengthRecorder)
	return send, receive
}

func manageQueue[T any](send <-chan T, receive chan<- T,
	lengthRecorder LengthRecorder) {
	queue := list.New()
	for {
		lengthRecorder(uint(queue.Len()))
		if front := queue.Front(); front == nil {
			if send == nil {
				close(receive)
				return
			}
			value, ok := <-send
			if !ok {
				close(receive)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case receive <- front.Value.(T):
				queue.Remove(front)
			case value, ok := <-send:
				if ok {
					queue.PushBack(value)
				} else {
					send = nil
				}
			}
		}
	}
}
