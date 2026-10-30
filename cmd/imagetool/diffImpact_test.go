package main

import (
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/triggers"
)

const testTriggers = `[
  {"MatchLines": ["^/etc/ssh/"], "Service": "sshd", "HighImpact": true},
  {"MatchLines": ["^/etc/rsyslog"], "Service": "rsyslog"},
  {"MatchLines": ["^/etc/motd"], "Service": "motd", "DoReload": true},
  {"MatchLines": ["^/boot/vmlinuz"], "Service": "kernel", "DoReboot": true}
]`

func TestMakeImpacts(t *testing.T) {
	trig, err := triggers.Decode([]byte(testTriggers))
	if err != nil {
		t.Fatal(err)
	}
	impacts := makeImpacts(trig.Triggers)
	expected := []struct {
		disruptive bool
		action     string
	}{
		{true, "reboot"},
		{false, "reload motd"},
		{false, "restart rsyslog"},
		{true, "restart sshd"},
	}
	if len(impacts) != len(expected) {
		t.Fatalf("got %d impacts, expected %d", len(impacts), len(expected))
	}
	for index, imp := range impacts {
		exp := expected[index]
		if imp.disruptive != exp.disruptive ||
			getAction(imp.trigger) != exp.action {
			t.Errorf("impact %d: got %v %s, expected %v %s", index,
				imp.disruptive, getAction(imp.trigger),
				exp.disruptive, exp.action)
		}
	}
}

func TestMakeImpactsReloadSkippedWhenRestarted(t *testing.T) {
	impacts := makeImpacts([]*triggers.Trigger{
		{Service: "ssh", DoReload: true},
		{Service: "ssh", HighImpact: true},
		{Service: "motd", DoReload: true},
	})
	if len(impacts) != 2 {
		t.Fatalf("got %d impacts, expected 2", len(impacts))
	}
	if action := getAction(impacts[0].trigger); action != "reload motd" {
		t.Errorf("got %s, expected reload motd", action)
	}
	if action := getAction(impacts[1].trigger); action != "restart ssh" {
		t.Errorf("got %s, expected restart ssh", action)
	}
	if !impacts[1].disruptive {
		t.Error("restart ssh should be disruptive")
	}
}

func TestMakeImpactsNoMatch(t *testing.T) {
	if impacts := makeImpacts(nil); len(impacts) != 0 {
		t.Errorf("got %d impacts, expected none", len(impacts))
	}
}
