package queue

import (
	"runtime"
	"testing"
	"time"
)

func receiveLoopValue(receiver Receiver[int], expected int, t *testing.T) {
	for ; true; time.Sleep(100 * time.Microsecond) {
		received, ok := receiver.Receive()
		if ok {
			if received != expected {
				t.Fatalf("got: %d, expected: %d", received, expected)
			}
			return
		}
	}
}

func TestQueueSimple(t *testing.T) {
	queue := newQueue[int](nil)
	queue.Send(0)
	queue.Send(1)
	queue.Send(2)
	queue.Send(3)
	queue.Send(4)
	runtime.Gosched()
	receiveLoopValue(queue, 0, t)
	receiveLoopValue(queue, 1, t)
	receiveLoopValue(queue, 2, t)
	receiveLoopValue(queue, 3, t)
	receiveLoopValue(queue, 4, t)
	if length := queue.queue.Len(); length != 0 {
		value := queue.queue.Front().Value.(int)
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
}

func TestQueueMany(t *testing.T) {
	numLoops := 12345
	queue := newQueue[int](nil)
	for count := 0; count < numLoops; count++ {
		queue.Send(count)
	}
	t.Logf("queue length: %d", queue.queue.Len())
	for count := 0; count < numLoops; count++ {
		receiveLoopValue(queue, count, t)
		if count%13 == 0 {
			runtime.Gosched()
		}
	}
	time.Sleep(time.Millisecond)
	if length := queue.queue.Len(); length != 0 {
		value := queue.queue.Front().Value.(int)
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
}

func TestQueuePulsing(t *testing.T) {
	queue := newQueue[int](nil)
	for pulseCount := 0; pulseCount < 123; pulseCount++ {
		for count := 0; count < 45; count++ {
			queue.Send(count * pulseCount)
		}
		time.Sleep(time.Millisecond)
		for count := 0; count < 45; count++ {
			receiveLoopValue(queue, count*pulseCount, t)
		}
	}
	t.Logf("queue length: %d", queue.queue.Len())
	if length := queue.queue.Len(); length != 0 {
		value := queue.queue.Front().Value.(int)
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
}

func TestQueueConcurrent(t *testing.T) {
	numLoops := 12345
	queue := newQueue[int](nil)
	go func() {
		for count := 0; count < numLoops; count++ {
			queue.Send(count)
			if count%7 == 0 {
				runtime.Gosched()
			}
		}
	}()
	for count := 0; count < numLoops; count++ {
		receiveLoopValue(queue, count, t)
		if count%13 == 0 {
			runtime.Gosched()
		}
	}
	time.Sleep(time.Millisecond)
	if length := queue.queue.Len(); length != 0 {
		value := queue.queue.Front().Value.(int)
		t.Errorf("queue has %d stuck entries, first: %d", length, value)
	}
}
