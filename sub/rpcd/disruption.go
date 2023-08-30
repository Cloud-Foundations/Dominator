package rpcd

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

const (
	intervalCheckChangeToDisrupt    = time.Second
	intervalCheckChangeToNonDisrupt = 5 * time.Second
	intervalCheckDisrupt            = 15 * time.Second
	intervalCheckNonDisrupt         = 5 * time.Minute
	intervalCheckStartup            = 10 * time.Second
	intervalCancelWhenPermitted     = 31 * time.Minute
	intervalCancelWhenRequested     = 15 * time.Minute
	intervalRequestWhenDenied       = time.Minute
	intervalRequestWhenRequested    = 15 * time.Minute
	intervalResendMinimum           = time.Second
	intervalResendSameMutation      = time.Minute
)

type runInfoType struct {
	command string
	state   proto.DisruptionState
}

type runResultType struct {
	command string
	err     error
	state   proto.DisruptionState
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
	t.disruptionManagerControl <- false
}

// This will grab the lock.
func (t *rpcType) disruptionRequest() proto.DisruptionState {
	if t.config.DisruptionManager == "" {
		return proto.DisruptionStateAnytime
	}
	t.rwLock.RLock()
	disruptionState := t.disruptionState
	t.rwLock.RUnlock()
	t.disruptionManagerControl <- true
	return disruptionState
}

func (t *rpcType) runDisruptionManager(command string) (
	proto.DisruptionState, error) {
	switch command {
	case disruptionManagerCancel, disruptionManagerRequest:
		t.params.Logger.Printf("Running: %s %s\n",
			t.config.DisruptionManager, command)
	default:
		t.params.Logger.Debugf(0, "Running: %s %s\n",
			t.config.DisruptionManager, command)
	}
	_output, err := exec.Command(t.config.DisruptionManager,
		command).CombinedOutput()
	if err == nil {
		return proto.DisruptionStatePermitted, nil
	}
	output := strings.TrimSpace(string(_output))
	e, ok := err.(*exec.ExitError)
	if !ok {
		if len(output) > 0 {
			return 0, fmt.Errorf("%s: %s", err, output)
		} else {
			return 0, fmt.Errorf("%s", err)
		}
	}
	switch e.ExitCode() {
	case 0:
		return proto.DisruptionStatePermitted, nil
	case 1:
		return proto.DisruptionStateRequested, nil
	case 2:
		return proto.DisruptionStateDenied, nil
	default:
		if len(output) > 0 {
			return 0,
				fmt.Errorf("invalid exit code: %d: %s", e.ExitCode(), output)
		} else {
			return 0, fmt.Errorf("invalid exit code: %d", e.ExitCode())
		}
	}
}

func (t *rpcType) startDisruptionManager() {
	if t.config.DisruptionManager == "" {
		return
	}
	commandChannel := make(chan string, 1)
	controlChannel := make(chan bool, 1)
	resultChannel := make(chan runInfoType, 1)
	t.disruptionManagerControl = controlChannel
	go t.disruptionManagerLoop(controlChannel, commandChannel, resultChannel)
	go t.disruptionManagerQueue(commandChannel, resultChannel)
}

func (t *rpcType) disruptionManagerLoop(controlChannel <-chan bool,
	commandChannel chan<- string, resultChannel <-chan runInfoType) {
	checkInterval := intervalCheckStartup
	checkTimer := time.NewTimer(0)
	var currentState proto.DisruptionState
	initialCancelTimer := time.NewTimer(intervalCancelWhenPermitted)
	var lastCommandTime time.Time
	var allowCancels, wantToDisrupt bool
	for {
		var resetCheckInterval bool
		select {
		case newWantToDisrupt := <-controlChannel:
			allowCancels = true
			clearTimer(initialCancelTimer)
			if newWantToDisrupt != wantToDisrupt {
				lastCommandTime = time.Time{}
				resetCheckInterval = true
			}
			wantToDisrupt = newWantToDisrupt
		case <-checkTimer.C:
			checkInterval += checkInterval >> 1
			if wantToDisrupt {
				if checkInterval > intervalCheckDisrupt {
					checkInterval = intervalCheckDisrupt
				}
			} else {
				if checkInterval > intervalCheckNonDisrupt {
					checkInterval = intervalCheckNonDisrupt
				}
			}
			commandChannel <- disruptionManagerCheck
			checkTimer.Reset(checkInterval)
		case <-initialCancelTimer.C:
			if !allowCancels {
				allowCancels = true
				lastCommandTime = time.Time{}
				resetCheckInterval = true
			}
		case result := <-resultChannel:
			if result.state != currentState {
				t.rwLock.Lock()
				t.disruptionState = result.state
				t.rwLock.Unlock()
				t.params.Logger.Printf(
					"Ran DisruptionManager(%s): %s->%s\n",
					result.command, currentState, result.state)
				currentState = result.state
				lastCommandTime = time.Time{}
				resetCheckInterval = true
			} else {
				t.params.Logger.Debugf(0, "Ran DisruptionManager(%s): %s\n",
					result.command, result.state)
			}
		}
		if wantToDisrupt {
			switch currentState {
			case proto.DisruptionStateRequested:
				if time.Since(lastCommandTime) > intervalRequestWhenRequested {
					commandChannel <- disruptionManagerRequest
					lastCommandTime = time.Now()
				}
			case proto.DisruptionStateDenied:
				if time.Since(lastCommandTime) > intervalRequestWhenDenied {
					commandChannel <- disruptionManagerRequest
					lastCommandTime = time.Now()
				}
			}
			if resetCheckInterval {
				checkInterval = intervalCheckChangeToDisrupt
				resetTimer(checkTimer, checkInterval)
			}
		} else if allowCancels {
			switch currentState {
			case proto.DisruptionStatePermitted:
				if time.Since(lastCommandTime) > intervalCancelWhenPermitted {
					commandChannel <- disruptionManagerCancel
					lastCommandTime = time.Now()
				}
			case proto.DisruptionStateRequested:
				if time.Since(lastCommandTime) > intervalCancelWhenRequested {
					commandChannel <- disruptionManagerCancel
					lastCommandTime = time.Now()
				}
			}
			if resetCheckInterval {
				checkInterval = intervalCheckChangeToNonDisrupt
				resetTimer(checkTimer, checkInterval)
			}
		}
	}
}

func (t *rpcType) disruptionManagerQueue(commandChannel <-chan string,
	resultChannel chan<- runInfoType) {
	commandIsRunning := false
	delayTimer := time.NewTimer(0)
	var lastCommandTime, lastMutatingCommandTime time.Time
	var lastMutatingCommand, nextCommand string
	runResultChannel := make(chan runResultType, 1)
	for {
		select {
		case <-delayTimer.C:
			if !commandIsRunning && nextCommand != "" {
				commandIsRunning = true
				go func(command string) {
					state, err := t.runDisruptionManager(command)
					runResultChannel <- runResultType{command, err, state}
				}(nextCommand)
				nextCommand = ""
			}
		case command := <-commandChannel:
			if command != disruptionManagerCheck &&
				command == lastMutatingCommand &&
				time.Since(lastMutatingCommandTime) <
					intervalResendSameMutation {
				continue
			}
			resetTimer(delayTimer,
				intervalResendMinimum-time.Since(lastCommandTime))
			if command != disruptionManagerCheck || nextCommand == "" {
				nextCommand = command
			}
		case runResult := <-runResultChannel:
			commandIsRunning = false
			lastCommandTime = time.Now()
			if runResult.err != nil {
				if runResult.command != disruptionManagerCheck &&
					nextCommand == "" {
					nextCommand = runResult.command
					resetTimer(delayTimer, time.Minute)
				}
				t.params.Logger.Printf("Error running DisruptionManager: %s\n",
					runResult.err)
			} else {
				if runResult.command != disruptionManagerCheck {
					lastMutatingCommand = runResult.command
					lastMutatingCommandTime = lastCommandTime
				}
				resultChannel <- runInfoType{runResult.command, runResult.state}
			}
		}
	}
}
