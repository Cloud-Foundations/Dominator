package concurrent

import (
	"errors"
	"testing"
	"time"
)

var (
	waitForReturn    = make(chan struct{}, 10)
	waitToGoodReturn = make(chan struct{}, 10)
	waitToBadReturn  = make(chan struct{}, 10)
)

func badFunc() error {
	<-waitToBadReturn
	waitForReturn <- struct{}{}
	return errors.New("injected error")
}

func goodFunc() error {
	<-waitToGoodReturn
	waitForReturn <- struct{}{}
	return nil
}

func TestReapAllGood(t *testing.T) {
	state := NewState(2)
	state.GoRun(goodFunc)
	state.GoRun(goodFunc)
	if len(waitForReturn) > 0 {
		t.Fatalf("Premature returns: %d", len(waitForReturn))
	}
	waitToGoodReturn <- struct{}{}
	waitToGoodReturn <- struct{}{}
	if err := state.Reap(); err != nil {
		t.Fatalf("Error reaping: %s", err)
	}
	if len(waitForReturn) != 2 {
		t.Fatalf("Expected 2 returns, got: %d", len(waitForReturn))
	}
	<-waitForReturn
	<-waitForReturn
}

func TestReapGoodAndBad(t *testing.T) {
	state := NewState(2)
	state.GoRun(goodFunc)
	state.GoRun(badFunc)
	if len(waitForReturn) > 0 {
		t.Fatalf("Premature returns: %d", len(waitForReturn))
	}
	waitToBadReturn <- struct{}{}
	reapError := make(chan error, 1)
	go func() {
		reapError <- state.Reap()
	}()
	timer := time.NewTimer(20 * time.Millisecond)
	select {
	case err := <-reapError:
		t.Fatalf("Premature reap, err: %s", err)
	case <-timer.C:
	}
	waitToGoodReturn <- struct{}{}
	if err := <-reapError; err == nil {
		t.Fatal("No error reaped")
	}
	if len(waitForReturn) != 2 {
		t.Fatalf("Expected 2 returns, got: %d", len(waitForReturn))
	}
}

func TestReapBadGoodGood(t *testing.T) {
	state := NewState(1)
	state.GoRun(badFunc)
	waitToBadReturn <- struct{}{}
	state.GoRun(goodFunc)
	waitToGoodReturn <- struct{}{}
	state.GoRun(goodFunc)
	waitToGoodReturn <- struct{}{}
	<-waitForReturn
	<-waitForReturn
	<-waitForReturn
	if err := state.Reap(); err == nil {
		t.Fatal("No error reaped")
	}
}
