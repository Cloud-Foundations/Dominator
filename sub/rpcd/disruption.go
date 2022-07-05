package rpcd

import (
	"fmt"
	"os/exec"
	"time"

	proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

type runResultType struct {
	err   error
	state proto.DisruptionState
}

func clearTimer(timer *time.Timer) {
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	clearTimer(timer)
	timer.Reset(duration)
}

// This must be called with the lock held.
func (t *rpcType) disruptionCancel() {
	if t.config.DisruptionManager == "" {
		return
	}
	switch t.disruptionState {
	case proto.DisruptionStatePermitted, proto.DisruptionStateRequested:
		t.disruptionManagerCommand <- disruptionManagerCancel
	}
}

// This will grab the lock.
func (t *rpcType) disruptionRequest() proto.DisruptionState {
	if t.config.DisruptionManager == "" {
		return proto.DisruptionStateAnytime
	}
	t.rwLock.RLock()
	disruptionState := t.disruptionState
	t.rwLock.RUnlock()
	if disruptionState == proto.DisruptionStateDenied {
		t.disruptionManagerCommand <- disruptionManagerRequest
	}
	return disruptionState
}

func (t *rpcType) runDisruptionManager(command string) (
	proto.DisruptionState, error) {
	switch command {
	case disruptionManagerCancel, disruptionManagerRequest:
		t.params.Logger.Printf("Running: %s %s\n",
			t.config.DisruptionManager, command)
	}
	err := exec.Command(t.config.DisruptionManager, command).Run()
	if err == nil {
		return proto.DisruptionStatePermitted, nil
	}
	e, ok := err.(*exec.ExitError)
	if !ok {
		return 0, err
	}
	switch e.ExitCode() {
	case 0:
		return proto.DisruptionStatePermitted, nil
	case 1:
		return proto.DisruptionStateRequested, nil
	case 2:
		return proto.DisruptionStateDenied, nil
	default:
		return 0, fmt.Errorf("invalid exit code: %d", e.ExitCode())
	}
}

func (t *rpcType) startDisruptionManager() {
	if t.config.DisruptionManager == "" {
		return
	}
	go t.disruptionManagerLoop()
}

func (t *rpcType) disruptionManagerLoop() {
	commandChannel := make(chan string, 1)
	t.disruptionManagerCommand = commandChannel
	checkInterval := time.Minute
	checkTimer := time.NewTimer(0)
	var disruptionState proto.DisruptionState
	reRequestTimer := time.NewTimer(time.Hour)
	clearTimer(reRequestTimer)
	var runningCommand string
	runResultChannel := make(chan runResultType, 1)
	var wantToDisrupt bool
	for {
		var command string
		select {
		case command = <-commandChannel:
			switch command {
			case disruptionManagerCancel:
				clearTimer(reRequestTimer)
				wantToDisrupt = false
			case disruptionManagerRequest:
				if disruptionState == proto.DisruptionStateDenied {
					resetTimer(reRequestTimer, time.Minute)
				} else {
					resetTimer(reRequestTimer, 15*time.Minute)
				}
				wantToDisrupt = true
			}
		case <-checkTimer.C:
			checkInterval += checkInterval >> 1
			if wantToDisrupt {
				if checkInterval > 15*time.Second {
					checkInterval = 15 * time.Second
				}
			} else {
				if checkInterval > 5*time.Minute {
					checkInterval = 5 * time.Minute
				}
			}
			command = disruptionManagerCheck
		case <-reRequestTimer.C:
			if wantToDisrupt {
				command = disruptionManagerRequest
				if runningCommand == "" {
					if disruptionState == proto.DisruptionStateDenied {
						resetTimer(reRequestTimer, time.Minute)
					} else {
						resetTimer(reRequestTimer, 15*time.Minute)
					}
				} else {
					resetTimer(reRequestTimer, 5*time.Second)
				}
			}
		case runResult := <-runResultChannel:
			if runResult.err != nil {
				t.params.Logger.Printf("Error running DisruptionManager: %s\n",
					runResult.err)
			} else {
				t.rwLock.Lock()
				t.disruptionState = runResult.state
				t.rwLock.Unlock()
				if runResult.state != disruptionState {
					if wantToDisrupt {
						checkInterval = time.Second
					} else {
						checkInterval = 5 * time.Second
					}
					t.params.Logger.Printf(
						"Ran DisruptionManager(%s): %s->%s\n",
						runningCommand, disruptionState, runResult.state)
				} else {
					t.params.Logger.Debugf(0, "Ran DisruptionManager(%s): %s\n",
						runningCommand, runResult.state)
				}
				disruptionState = runResult.state
			}
			runningCommand = ""
		}
		if runningCommand == "" && command != "" {
			go func(command string) {
				state, err := t.runDisruptionManager(command)
				runResultChannel <- runResultType{err, state}
			}(command)
			runningCommand = command
		}
		switch command {
		case disruptionManagerCancel:
			checkInterval = 5 * time.Second
		case disruptionManagerRequest:
			checkInterval = time.Second
		}
		resetTimer(checkTimer, checkInterval)
	}
}
