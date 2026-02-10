package queue

import (
	"runtime"
	"testing"
	"time"
)

func receiveValue(channel <-chan int, expected int, t *testing.T) {
	received := <-channel
	if received != expected {
		t.Fatalf("got: %d, expected: %d", received, expected)
	}
}

func TestReceiveSimple(t *testing.T) {
	sender, channel := newReceiveChannel[int](2, nil)
	sender.Send(0)
	sender.Send(1)
	sender.Send(2)
	sender.Send(3)
	sender.Send(4)
	runtime.Gosched()
	receiveValue(channel, 0, t)
	receiveValue(channel, 1, t)
	receiveValue(channel, 2, t)
	receiveValue(channel, 3, t)
	receiveValue(channel, 4, t)
	if length := sender.queue.Length(); length != 0 {
		value := sender.queue.Front().Value()
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
	if len(channel) != 0 {
		t.Errorf("channel has %d stuck entries", len(channel))
	}
}

func TestReceiveMany(t *testing.T) {
	numLoops := 12345
	sender, channel := newReceiveChannel[int](2, nil)
	for count := 0; count < numLoops; count++ {
		sender.Send(count)
	}
	t.Logf("channel size: %d, queue length: %d",
		len(channel), sender.queue.Length())
	for count := 0; count < numLoops; count++ {
		receiveValue(channel, count, t)
		if count%13 == 0 {
			runtime.Gosched()
		}
	}
	time.Sleep(time.Millisecond)
	if length := sender.queue.Length(); length != 0 {
		value := sender.queue.Front().Value()
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
	if len(channel) != 0 {
		t.Errorf("channel has %d stuck entries", len(channel))
	}
}

func TestReceivePulsing(t *testing.T) {
	sender, channel := newReceiveChannel[int](2, nil)
	for pulseCount := 0; pulseCount < 123; pulseCount++ {
		for count := 0; count < 45; count++ {
			sender.Send(count * pulseCount)
		}
		time.Sleep(time.Millisecond)
		for count := 0; count < 45; count++ {
			receiveValue(channel, count*pulseCount, t)
		}
	}
	t.Logf("channel size: %d, queue length: %d",
		len(channel), sender.queue.Length())
	if length := sender.queue.Length(); length != 0 {
		value := sender.queue.Front().Value()
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
	if len(channel) != 0 {
		t.Errorf("channel has %d stuck entries", len(channel))
	}
}

func TestReceiveConcurrent(t *testing.T) {
	numLoops := 12345
	sender, channel := newReceiveChannel[int](2, nil)
	go func() {
		for count := 0; count < numLoops; count++ {
			sender.Send(count)
			if count%7 == 0 {
				runtime.Gosched()
			}
		}
	}()
	for count := 0; count < numLoops; count++ {
		receiveValue(channel, count, t)
		if count%13 == 0 {
			runtime.Gosched()
		}
	}
	time.Sleep(time.Millisecond)
	if length := sender.queue.Length(); length != 0 {
		value := sender.queue.Front().Value()
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
	if len(channel) != 0 {
		t.Errorf("channel has %d stuck entries", len(channel))
	}
}
