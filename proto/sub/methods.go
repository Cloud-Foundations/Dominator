package sub

import (
	"errors"
	"fmt"
)

const (
	disruptionRequestUnknown = "UNKNOWN DisruptionRequest"
	disruptionStateUnknown   = "UNKNOWN DisruptionState"
)

var (
	disruptionRequestToText = map[DisruptionRequest]string{
		DisruptionRequestCheck:   "check",
		DisruptionRequestRequest: "request",
		DisruptionRequestCancel:  "cancel",
	}
	textToDisruptionRequest map[string]DisruptionRequest

	disruptionStateToText = map[DisruptionState]string{
		DisruptionStateAnytime:   "anytime",
		DisruptionStatePermitted: "permitted",
		DisruptionStateRequested: "requested",
		DisruptionStateDenied:    "denied",
	}
	textToDisruptionState map[string]DisruptionState
)

func init() {
	textToDisruptionRequest = make(map[string]DisruptionRequest,
		len(disruptionRequestToText))
	for request, text := range disruptionRequestToText {
		textToDisruptionRequest[text] = request
	}
	textToDisruptionState = make(map[string]DisruptionState,
		len(disruptionStateToText))
	for state, text := range disruptionStateToText {
		textToDisruptionState[text] = state
	}
}

func (disruptionRequest *DisruptionRequest) CheckValid() error {
	if _, ok := disruptionRequestToText[*disruptionRequest]; !ok {
		return errors.New(disruptionRequestUnknown)
	} else {
		return nil
	}
}

func (disruptionRequest DisruptionRequest) MarshalText() ([]byte, error) {
	if text := disruptionRequest.String(); text == disruptionRequestUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (disruptionRequest *DisruptionRequest) Set(value string) error {
	if val, ok := textToDisruptionRequest[value]; !ok {
		return errors.New(disruptionRequestUnknown)
	} else {
		*disruptionRequest = val
		return nil
	}
}

func (disruptionRequest DisruptionRequest) String() string {
	if str, ok := disruptionRequestToText[disruptionRequest]; !ok {
		return disruptionRequestUnknown
	} else {
		return str
	}
}

func (disruptionRequest *DisruptionRequest) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToDisruptionRequest[txt]; ok {
		*disruptionRequest = val
		return nil
	} else {
		return errors.New("unknown DisruptionRequest: " + txt)
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
		return disruptionStateUnknown
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
