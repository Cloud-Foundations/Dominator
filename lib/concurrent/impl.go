package concurrent

import (
	"runtime"
)

func newState(numConcurrent uint, p putter) *State {
	state := &State{errorChannel: make(chan error), putter: p}
	if numConcurrent > 0 {
		state.semaphore = make(chan struct{}, numConcurrent)
	} else {
		state.semaphore = make(chan struct{}, runtime.NumCPU())
	}
	return state
}

func (*nilPutter) put() {
}

func (state *State) goRun(doFunc func() error) error {
	if state.entered {
		panic("GoRun is not re-entrant safe")
	}
	if state.reaped {
		panic("state has been reaped")
	}
	state.entered = true
	defer func() { state.entered = false }()
	for {
		select {
		case err := <-state.errorChannel:
			state.pending--
			if err != nil {
				state.reap()
				return err
			}
		case state.semaphore <- struct{}{}:
			state.pending++
			go func() {
				state.errorChannel <- doFunc()
				<-state.semaphore
				state.putter.put()
			}()
			return nil
		}
	}
}

func (state *State) reap() error {
	state.reaped = true
	close(state.semaphore)
	var err error
	for ; state.pending > 0; state.pending-- {
		if e := <-state.errorChannel; err == nil && e != nil {
			err = e
		}
	}
	close(state.errorChannel)
	return err
}
