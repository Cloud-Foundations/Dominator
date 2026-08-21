package triggers

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/pathregexp"
)

type keyType struct {
	doReload    bool
	serviceName string
}

type MergeableTriggers struct {
	triggers map[keyType]*mergeableTrigger
}

type mergeableTrigger struct {
	matchLines map[string]struct{}
	doReboot   bool
	highImpact bool
}

type Trigger struct {
	MatchLines   []string
	matchRegexes []pathregexp.Regexp
	Service      string // Name of service.
	SortName     string `json:",omitempty"` // Control order of triggers run.
	DoReboot     bool   `json:",omitempty"` // If true, reboot after start.
	DoReload     bool   `json:",omitempty"` // If true, only reload the service.
	HighImpact   bool   `json:",omitempty"` // If true, trigger is disruptive.
}

func (trigger *Trigger) RegisterStrings(registerFunc func(string)) {
	trigger.registerStrings(registerFunc)
}

func (trigger *Trigger) ReplaceStrings(replaceFunc func(string) string) {
	trigger.replaceStrings(replaceFunc)
}

type Triggers struct {
	Triggers          []*Trigger
	compiled          bool
	matchedTriggers   map[*Trigger]struct{}
	unmatchedTriggers map[*Trigger]struct{}
}

func Decode(jsonData []byte) (*Triggers, error) {
	return decode(jsonData)
}

func Load(filename string) (*Triggers, error) {
	return load(filename)
}

func Read(reader io.Reader) (*Triggers, error) {
	return read(reader)
}

func (mt *MergeableTriggers) ExportTriggers() *Triggers {
	return mt.exportTriggers()
}

func (mt *MergeableTriggers) Merge(triggers *Triggers) {
	mt.merge(triggers)
}

func New() *Triggers {
	return newTriggers()
}

// Compile will compile the regular expressions for later use.
func (triggers *Triggers) Compile() error {
	return triggers.compile()
}

// Clone will return a shallow copy of the triggers, with separate internal
// state for matching.
func (triggers *Triggers) Clone() *Triggers {
	return triggers.clone()
}

// GetMatchedStatistics will return the number of matched and unmatched
// triggers. This must not be called concurrently with itself or other methods
// related to matching
func (triggers *Triggers) GetMatchStatistics() (nMatched, nUnmatched uint) {
	return triggers.getMatchStatistics()
}

// GetMatchedTriggers will return the list of matched triggers and clear the
// internal match state. This must not be called concurrently with itself or
// other methods related to matching
func (triggers *Triggers) GetMatchedTriggers() []*Trigger {
	return triggers.getMatchedTriggers()
}

// Match will check if the specified line matches any of the triggers. Internal
// state is updated. This must not be called concurrently with itself or other
// methods related to matching.
func (triggers *Triggers) Match(line string) {
	triggers.match(line)
}

func (triggers *Triggers) Len() int {
	return len(triggers.Triggers)
}

func (triggers *Triggers) Less(left, right int) bool {
	return triggers.less(left, right)
}

func (triggers *Triggers) RegisterStrings(registerFunc func(string)) {
	triggers.registerStrings(registerFunc)
}

func (triggers *Triggers) ReplaceStrings(replaceFunc func(string) string) {
	triggers.replaceStrings(replaceFunc)
}

func (triggers *Triggers) Swap(left, right int) {
	triggers.Triggers[left], triggers.Triggers[right] =
		triggers.Triggers[right], triggers.Triggers[left]
}
