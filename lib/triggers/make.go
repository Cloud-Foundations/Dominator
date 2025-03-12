package triggers

import (
	"github.com/Cloud-Foundations/Dominator/lib/pathregexp"
)

func newTriggers() *Triggers {
	return &Triggers{}
}

func (triggers *Triggers) compile() error {
	if triggers.compiled {
		return nil
	}
	for _, trigger := range triggers.Triggers {
		trigger.matchRegexes = make([]pathregexp.Regexp,
			len(trigger.MatchLines))
		for index, line := range trigger.MatchLines {
			var err error
			trigger.matchRegexes[index], err = pathregexp.Compile(line)
			if err != nil {
				return err
			}
		}
	}
	triggers.compiled = true
	return nil
}
