package lib

import (
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
)

func checkImpact(triggerList []*triggers.Trigger) (highImpact, reboot bool) {
	for _, trg := range triggerList {
		if trg.DoReboot {
			reboot = true
		}
		if trg.HighImpact {
			highImpact = true
		}
	}
	return
}
