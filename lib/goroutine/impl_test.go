package goroutine

import (
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestQuitAndRun(t *testing.T) {
	g := New()
	g.Quit()
	defer func() { recover() }()
	g.Run(func() {})
	t.Fatal("Run-after-Quit did not panic")
}

func TestRun(t *testing.T) {
	g := New()
	defer g.Quit()
	var finished, started bool
	g.Run(func() {
		started = true
		time.Sleep(time.Millisecond * 10)
		finished = true
	})
	runtime.Gosched()
	if !started {
		t.Fatal("Function not started")
	}
	if !finished {
		t.Fatal("Function not finished")
	}
}

func TestStart(t *testing.T) {
	g := New()
	defer g.Quit()
	var firstFinished, firstStarted bool
	g.Start(func() {
		firstStarted = true
		time.Sleep(time.Millisecond * 10)
		firstFinished = true
	})
	runtime.Gosched()
	if !firstStarted {
		t.Fatal("First function not started")
	}
	if firstFinished {
		t.Fatal("First function finished")
	}
	secondStarted := false
	g.Start(func() {
		secondStarted = true
	})
	if !firstFinished {
		t.Fatal("First function not finished")
	}
	runtime.Gosched()
	if !secondStarted {
		t.Fatal("Second function not started")
	}
}

func TestStartQuit(t *testing.T) {
	g := New()
	var finished bool
	ch := make(chan struct{}, 1)
	g.Start(func() {
		ch <- struct{}{} // Signal I'm running.
		time.Sleep(time.Millisecond * 10)
		finished = true
	})
	<-ch // Wait for goroutine to start.
	g.Quit()
	if !finished {
		t.Fatal("Function not finished")
	}
}

func TestWait(t *testing.T) {
	g := New()
	defer g.Quit()
	var finished, started bool
	g.Start(func() {
		started = true
		time.Sleep(time.Millisecond * 10)
		finished = true
	})
	runtime.Gosched()
	if !started {
		t.Fatal("Function not started")
	}
	if finished {
		t.Fatal("Function finished")
	}
	g.Wait()
	if !finished {
		t.Fatal("Function not finished")
	}
}

func TestWithThreadLocking(t *testing.T) {
	g := New()
	defer g.Quit()
	g.Run(runtime.LockOSThread)
	var gTid int
	g.Run(func() { gTid = syscall.Gettid() })
	callerTid := syscall.Gettid()
	if gTid == callerTid {
		t.Fatal("Caller and goroutine threads are the same")
	}
}
