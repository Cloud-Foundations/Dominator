package sub

const stateUnknown = "UNKNOWN State"

var (
	disruptionStateToText = map[DisruptionState]string{
		DisruptionStateAnytime:   "anytime",
		DisruptionStatePermitted: "permitted",
		DisruptionStateRequested: "requested",
		DisruptionStateDenied:    "denied",
	}
)

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
