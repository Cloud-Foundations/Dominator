package queue

import (
	"fmt"
	"testing"
	"time"
)

func doBulkSendBulkReceive(send chan<- int, receive <-chan int) error {
	// Bulk send.
	for index := range 100 {
		send <- index
	}
	// Bulk receive.
	for index := range 100 {
		if value := <-receive; value != index {
			return fmt.Errorf("expected: %d got: %d", index, value)
		}
	}
	return nil
}

func TestBulkSendBulkReceive(t *testing.T) {
	send, receive := NewChannelPair[int](nil)
	timer := time.NewTimer(time.Second)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- doBulkSendBulkReceive(send, receive)
	}()
	select {
	case err := <-errChannel:
		if err != nil {
			t.Fatal(err)
		}
	case <-timer.C:
		t.Fatal("timed out waiting for completion")
	}
}
