/*
Package concurrent provides a simple way to run functions concurrently and
then reap the results.

Package concurrent allows cuncurrent running of provided functions, by
default limiting the parallelism to the number of CPUs. The functions return
an error value and these may be reaped at the end.
*/
package concurrent

import (
	"sync"
	"time"
)

type nilPutter struct{}

type putter interface {
	put()
}

type AutoScaler struct {
	entered                  bool
	error                    error
	errorChannel             chan error
	forcedConcurrent         bool
	pending                  uint64
	reaped                   bool
	semaphore                chan struct{}
	mutex                    sync.Mutex // Protect everything below.
	accumulatingWork         uint64
	blockedThreads           uint
	lastRecalculation        time.Time
	previousAvailableThreads uint
	previousWorkRate         float64
}

type MeasuringRunner interface {
	GoRun(doFunc func() (uint64, error)) error
	Reap() error
}

type SimpleRunner interface {
	GoRun(doFunc func() error) error
	Reap() error
}

var (
	// Interface checks.
	_ MeasuringRunner = (*AutoScaler)(nil)
	_ SimpleRunner    = (*State)(nil)
)

// NewAutoScaler returns a new AutoScaler state which can be used to run
// goroutines concurrently, adjusting the number of goroutines until peak
// throughput is acheived. The maximum number of goroutines is specified by
// maxConcurrent (runtime.NumCPU() if zero). If maxConcurrent is 1 then no
// goroutines are created and work is performed synchronously.
func NewAutoScaler(maxConcurrent uint) *AutoScaler {
	return newAutoScaler(maxConcurrent)
}

// GoRun will run the provided function in a goroutine. It must return a value
// indicating the amount of work done (used in estimating throughput) and an
// error. If the function returns a non-nil error, this will be returned in a
// future call to GoRun or by Reap. GoRun cannot be called concurrently with
// GoRun or Reap.
func (state *AutoScaler) GoRun(doFunc func() (uint64, error)) error {
	return state.goRun(doFunc)
}

// Reap returns the first error encountered by the functions and waits for
// remaining functions to complete. The AutoScaler can no longer be used after
// Reap.
func (state *AutoScaler) Reap() error {
	if state.entered {
		panic("GoRun is running")
	}
	return state.reap()
}

// State maintains state needed to manage running functions concurrently.
type State struct {
	entered      bool
	error        error
	errorChannel chan error
	pending      uint64
	putter       putter
	reaped       bool
	semaphore    chan struct{}
}

// NewState returns a new State.
func NewState(numConcurrent uint) *State {
	return newState(numConcurrent, &nilPutter{})
}

func NewStateWithLinearConcurrencyIncrease(initialNumConcurrent uint,
	maximumNumConcurrent uint) *State {
	return newStateWithLinearConcurrencyIncrease(initialNumConcurrent,
		maximumNumConcurrent)
}

// GoRun will run the provided function in a goroutine. If the function returns
// a non-nil error, this will be returned in a future call to GoRun or by
// Reap. GoRun cannot be called concurrently with GoRun or Reap.
func (state *State) GoRun(doFunc func() error) error {
	return state.goRun(doFunc)
}

// Reap returns the first error encountered by the functions and waits for
// remaining functions to complete. The State can no longer be used after Reap.
func (state *State) Reap() error {
	if state.entered {
		panic("GoRun is running")
	}
	return state.reap()
}
