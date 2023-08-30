package sub

import (
	"fmt"
)

const stateUnknown = "UNKNOWN State"

var (
	disruptionStateToText = map[DisruptionState]string{
		DisruptionStateAnytime:   "anytime",
		DisruptionStatePermitted: "permitted",
		DisruptionStateRequested: "requested",
		DisruptionStateDenied:    "denied",
	}
	textToDisruptionState map[string]DisruptionState
)

func init() {
	textToDisruptionState = make(map[string]DisruptionState,
		len(disruptionStateToText))
	for state, text := range disruptionStateToText {
		textToDisruptionState[text] = state
	}
}

func VerifyDisruptionState(state DisruptionState) bool {
	_, ok := disruptionStateToText[state]
	return ok
}

func (state DisruptionState) String() string {
	if text, ok := disruptionStateToText[state]; ok {
		return text
	} else {
		return stateUnknown
	}
}

func (state DisruptionState) MarshalText() (text []byte, err error) {
	if text, ok := disruptionStateToText[state]; ok {
		return []byte(text), nil
	} else {
		return nil, fmt.Errorf("invalid DisruptionState: %d", state)
	}
}

func (state *DisruptionState) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToDisruptionState[txt]; ok {
		*state = val
		return nil
	} else {
		return fmt.Errorf("unknown DisruptionState: %s", txt)
	}
}
