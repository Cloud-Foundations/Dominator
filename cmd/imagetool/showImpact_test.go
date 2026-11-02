package main

import (
	"reflect"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/triggers"
)

const testTriggers = `[
  {"MatchLines": ["^/etc/ssh/"], "Service": "sshd", "HighImpact": true},
  {"MatchLines": ["^/etc/rsyslog"], "Service": "rsyslog"},
  {"MatchLines": ["^/etc/motd"], "Service": "motd", "DoReload": true},
  {"MatchLines": ["^/boot/vmlinuz"], "Service": "kernel", "DoReboot": true}
]`

func TestSortByImpact(t *testing.T) {
	trig, err := triggers.Decode([]byte(testTriggers))
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"kernel", "sshd", "rsyslog", "motd"}
	checkServices(t, sortByImpact(trig.Triggers), expected)
}

func TestSortByImpactReloadSkippedWhenRestarted(t *testing.T) {
	impacts := sortByImpact([]*triggers.Trigger{
		{Service: "ssh", DoReload: true},
		{Service: "ssh", HighImpact: true},
		{Service: "motd", DoReload: true},
	})
	checkServices(t, impacts, []string{"ssh", "motd"})
}

func TestSortByImpactNoMatch(t *testing.T) {
	if impacts := sortByImpact(nil); len(impacts) != 0 {
		t.Errorf("got %d impacts, expected none", len(impacts))
	}
}

func checkServices(t *testing.T, impacts []*triggers.Trigger,
	expected []string) {
	var services []string
	for _, trg := range impacts {
		services = append(services, trg.Service)
	}
	if !reflect.DeepEqual(services, expected) {
		t.Errorf("got %v, expected %v", services, expected)
	}
}
