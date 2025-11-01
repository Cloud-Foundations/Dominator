package triggers

import (
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
)

func (mt *MergeableTriggers) exportTriggers() *Triggers {
	if len(mt.triggers) < 1 {
		return nil
	}
	triggerList := make([]*Trigger, 0, len(mt.triggers))
	for key, trigger := range mt.triggers {
		matchLines := stringutil.ConvertMapKeysToList(trigger.matchLines, true)
		triggerList = append(triggerList, &Trigger{
			MatchLines: matchLines,
			Service:    key.serviceName,
			DoReboot:   trigger.doReboot,
			DoReload:   key.doReload,
			HighImpact: trigger.highImpact,
		})
	}
	triggers := New()
	triggers.Triggers = triggerList
	sort.Sort(triggers)
	return triggers
}

func (mt *MergeableTriggers) merge(triggers *Triggers) {
	if triggers == nil || len(triggers.Triggers) < 1 {
		return
	}
	if mt.triggers == nil {
		mt.triggers = make(map[keyType]*mergeableTrigger,
			len(triggers.Triggers))
	}
	for _, trigger := range triggers.Triggers {
		key := keyType{trigger.DoReload, trigger.Service}
		trig := mt.triggers[key]
		if trig == nil {
			trig = new(mergeableTrigger)
			trig.matchLines = make(map[string]struct{})
			mt.triggers[key] = trig
		}
		for _, matchLine := range trigger.MatchLines {
			trig.matchLines[matchLine] = struct{}{}
		}
		if trigger.DoReboot {
			trig.doReboot = true
		}
		if trigger.HighImpact {
			trig.highImpact = true
		}
	}
}
