package concurrent

import (
	"os"
	"runtime"
	"strconv"
	"time"
)

const recalculateInterval = time.Millisecond

func newAutoScaler(maxConcurrent uint) *AutoScaler {
	if maxConcurrent < 1 {
		maxConcurrent = uint(runtime.NumCPU())
	}
	var forcedConcurrent uint
	envVal, err := strconv.Atoi(os.Getenv("AUTO_SCALER_FORCE_CONCURRENT"))
	if err == nil && envVal > 0 {
		forcedConcurrent = uint(envVal)
	}
	if forcedConcurrent > 0 {
		maxConcurrent = forcedConcurrent
	}
	if maxConcurrent == 1 {
		return &AutoScaler{}
	}
	state := &AutoScaler{
		errorChannel:   make(chan error, 1),
		semaphore:      make(chan struct{}, maxConcurrent),
		blockedThreads: maxConcurrent - 1, // Start slow.
	}
	if forcedConcurrent > 0 {
		state.forcedConcurrent = true
	} else {
		for range state.blockedThreads {
			state.semaphore <- struct{}{}
		}
	}
	return state
}

func (state *AutoScaler) adjustConcurrency(workDone uint64) {
	if state.blockedThreads < 1 { // Fast check to see if we're at full speed.
		return
	}
	if state.forcedConcurrent {
		return
	}
	var reduceConcurrency bool
	state.mutex.Lock()
	defer func() {
		state.mutex.Unlock()
		if reduceConcurrency {
			state.semaphore <- struct{}{}
		}
	}()
	if state.blockedThreads < 1 { // Correctness check.
		return
	}
	now := time.Now()
	interval := now.Sub(state.lastRecalculation)
	if interval < recalculateInterval {
		state.accumulatingWork += workDone
		return
	}
	availableThreads := uint(cap(state.semaphore)) - state.blockedThreads
	workRate := float64(state.accumulatingWork) / float64(interval)
	if workRate > state.previousWorkRate {
		state.blockedThreads--
		<-state.semaphore
	} else if availableThreads >= state.previousAvailableThreads &&
		availableThreads > 1 &&
		workRate < state.previousWorkRate*0.75 {
		state.blockedThreads++
		reduceConcurrency = true
	}
	state.accumulatingWork = 0
	state.previousAvailableThreads = availableThreads
	state.previousWorkRate = workRate
	state.lastRecalculation = now
}

func (state *AutoScaler) goRun(doFunc func() (uint64, error)) error {
	if state.entered {
		panic("GoRun is not re-entrant safe")
	}
	if state.error != nil {
		return state.error
	}
	if state.reaped {
		panic("state has been reaped")
	}
	state.entered = true
	defer func() { state.entered = false }()
	if state.semaphore == nil { // No concurrency: run synchronously.
		if _, err := doFunc(); err != nil {
			state.error = err
		}
		return state.error
	}
	for {
		select {
		case err := <-state.errorChannel:
			state.pending--
			if err != nil {
				state.error = err
				state.reap()
				return err
			}
		case state.semaphore <- struct{}{}:
			state.pending++
			go func() {
				workDone, err := doFunc()
				state.errorChannel <- err
				<-state.semaphore
				if err == nil {
					state.adjustConcurrency(workDone)
				}
			}()
			return nil
		}
	}
}

func (state *AutoScaler) reap() error {
	if state.reaped {
		return state.error
	}
	state.reaped = true
	if state.semaphore == nil {
		return state.error
	}
	close(state.semaphore)
	err := state.error
	for ; state.pending > 0; state.pending-- {
		if e := <-state.errorChannel; err == nil && e != nil {
			err = e
		}
	}
	close(state.errorChannel)
	return err
}
