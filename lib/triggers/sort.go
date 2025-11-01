package triggers

func (triggers *Triggers) less(left, right int) bool {
	if triggers.Triggers[left].Service == triggers.Triggers[right].Service {
		return triggers.Triggers[left].DoReload
	}
	return triggers.Triggers[left].Service < triggers.Triggers[right].Service
}
