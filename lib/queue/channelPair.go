package queue

import "github.com/Cloud-Foundations/Dominator/lib/list"

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
	queue := list.New[T]()
	for {
		lengthRecorder(uint(queue.Length()))
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
			case receive <- front.Value():
				front.Remove()
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
