package concurrent

import (
	"testing"
	"time"
)

func TestReapAndWaitAutoScaler(t *testing.T) {
	state := NewAutoScaler(8)
	var finished bool
	state.GoRun(func() (uint64, error) {
		time.Sleep(2 * recalculateInterval)
		return 1000000, nil
	})
	state.GoRun(func() (uint64, error) {
		time.Sleep(4 * recalculateInterval)
		return 2000000, nil
	})
	state.GoRun(func() (uint64, error) {
		time.Sleep(6 * recalculateInterval)
		return 3000000, nil
	})
	state.GoRun(func() (uint64, error) {
		time.Sleep(8 * recalculateInterval)
		finished = true
		return 1000, nil
	})
	if err := state.Reap(); err != nil {
		t.Fatalf("Error reaping: %s", err)
	}
	if !finished {
		t.Fatal("goroutine not finished")
	}
}
