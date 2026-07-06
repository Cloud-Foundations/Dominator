package queue

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

type pairType struct {
	position uint64
	value    int
}

func testConsumer(t *testing.T, subscriber <-chan BroadcastEntry[int],
	numEntries, offsetIndex int, offsetPosition uint64) {
	for index := range numEntries {
		runtime.Gosched()
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case entry := <-subscriber:
			if entry.Value != index+offsetIndex {
				t.Fatalf("[%d] got: %d, expected: %d",
					entry.Position, entry.Value, index)
			}
			if v := uint64(entry.Value) + offsetPosition; v != entry.Position {
				t.Fatalf("[%d] value: %d",
					entry.Position, v)
			}
		case <-timer.C:
			t.Fatalf("timed out receiving: %d", index)
		}
	}
}

func TestClosedSubscriber(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 0)
	subscriber := queue.Subscribe(0)
	queue.CloseSubscriber(subscriber)
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	entry, ok := <-subscriber
	if ok {
		t.Fatalf("got: %d, expected closure", entry.Value)
	}
}

func TestSingleEarlySubscriber(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 0)
	subscriber := queue.Subscribe(0)
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	testConsumer(t, subscriber, numEntries, 0, 0)
}

func TestSingleLateSubscriber(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 0)
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	testConsumer(t, queue.Subscribe(0), numEntries, 0, 0)
}

func TestSinglePartialSubscriber(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 0)
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	testConsumer(t, queue.Subscribe(3), 7, 3, 0)
}

func TestMultipleSubscribers(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 0)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	wg.Add(1)
	numEntries := 100
	go func() {
		testConsumer(t, queue.Subscribe(0), numEntries, 0, 0)
		wg.Done()
	}()
	go func() {
		testConsumer(t, queue.Subscribe(0), numEntries, 0, 0)
		wg.Done()
	}()
	for index := range numEntries {
		runtime.Gosched()
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	wg.Wait()
}

func TestWithInitial(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](17, 0)
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	testConsumer(t, queue.Subscribe(0), numEntries, 0, 17)
}

func TestWithRemovals(t *testing.T) {
	queue, sender, removals := NewBroadcastQueue[int](0, 4)
	wg := &sync.WaitGroup{}
	wg.Add(6)
	go func() {
		for range removals {
			wg.Done()
		}
	}()
	numEntries := 10
	for index := range numEntries {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case sender <- index:
			timer.Reset(0)
		case <-timer.C:
			t.Fatalf("timed out sending: %d", index)
		}
	}
	wg.Wait()
	testConsumer(t, queue.Subscribe(0), 4, 6, 0)
}

func TestIterators(t *testing.T) {
	queue, sender, _ := NewBroadcastQueue[int](0, 4)
	sender <- 3
	sender <- 5
	sender <- 7
	sender <- 11
	queue.Sync()
	expectedList := []pairType{
		{0, 3},
		{1, 5},
		{2, 7},
		{3, 11},
	}
	index := 0
	queue.IterateEntries(func(entry BroadcastEntry[int]) bool {
		expected := expectedList[index]
		if entry.Position != expected.position {
			t.Errorf("IterateEntries[%d]: position: %d != %d",
				index, entry.Position, expected.position)
		}
		if entry.Value != expected.value {
			t.Errorf("IterateEntries[%d]: value: %d != %d",
				index, entry.Value, expected.value)
		}
		index++
		return true
	})
	index = 0
	queue.IterateValues(func(value int) bool {
		expected := expectedList[index]
		if value != expected.value {
			t.Errorf("IterateEntries[%d]: value: %d != %d",
				index, value, expected.value)
		}
		index++
		return true
	})
	index = 0
	queue.IterateEntriesReverse(func(entry BroadcastEntry[int]) bool {
		expected := expectedList[len(expectedList)-index-1]
		if entry.Position != expected.position {
			t.Errorf("IterateEntries[%d]: position: %d != %d",
				index, entry.Position, expected.position)
		}
		if entry.Value != expected.value {
			t.Errorf("IterateEntries[%d]: value: %d != %d",
				index, entry.Value, expected.value)
		}
		index++
		return true
	})
	index = 0
	queue.IterateValuesReverse(func(value int) bool {
		expected := expectedList[len(expectedList)-index-1]
		if value != expected.value {
			t.Errorf("IterateEntries[%d]: value: %d != %d",
				index, value, expected.value)
		}
		index++
		return true
	})
}
