package triggers

func (trigger *Trigger) registerStrings(registerFunc func(string)) {
	for _, str := range trigger.MatchLines {
		registerFunc(str)
	}
	registerFunc(trigger.Service)
}

func (trigger *Trigger) replaceStrings(replaceFunc func(string) string) {
	for index, str := range trigger.MatchLines {
		trigger.MatchLines[index] = replaceFunc(str)
	}
	trigger.Service = replaceFunc(trigger.Service)
}

func (triggers *Triggers) registerStrings(registerFunc func(string)) {
	if triggers != nil {
		for _, trigger := range triggers.Triggers {
			trigger.RegisterStrings(registerFunc)
		}
	}
}

func (triggers *Triggers) replaceStrings(replaceFunc func(string) string) {
	if triggers != nil {
		for _, trigger := range triggers.Triggers {
			trigger.ReplaceStrings(replaceFunc)
		}
	}
}
