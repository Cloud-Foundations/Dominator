package filter

import (
	"testing"
)

var (
	excludeFilterLines = []string{
		"/.*app-log.*",
		"/etc/fstab",
		"/file.log",
		"/foo(|.*)$",
		"/tmp(|/.*)$",
	}

	includeFilterLines = []string{
		"!",
		"/bin(|/.*)$",
	}
)

func TestExclude(t *testing.T) {
	filt, err := New(excludeFilterLines)
	if err != nil {
		t.Error(err)
	}
	expectedNonMatches := []string{
		"/.myprog-log.err",
		"/bin",
		"/etc",
		"/etc/passwd",
		"/tmpfile",
	}
	for _, line := range expectedNonMatches {
		if filt.Match(line) {
			t.Errorf("\"%s\" should not have matched", line)
		}
	}
	expectedMatches := []string{
		"/.myapp-log.err",
		"/.myapp-logout",
		"/etc/fstab",
		"/file.log",
		"/file%log",
		"/foo",
		"/foobar",
		"/foo/bar",
		"/tmp",
		"/tmp/file",
	}
	for _, line := range expectedMatches {
		if !filt.Match(line) {
			t.Errorf("\"%s\" should have matched", line)
		}
	}
}

func TestInverted(t *testing.T) {
	filt, err := New(includeFilterLines)
	if err != nil {
		t.Error(err)
	}
	expectedNonMatches := []string{
		"/bin",
		"/bin/ls",
	}
	for _, line := range expectedNonMatches {
		if filt.Match(line) {
			t.Errorf("\"%s\" should not have matched", line)
		}
	}
	expectedMatches := []string{
		"/bingo",
		"/etc/fstab",
		"/tmp",
		"/tmp/file",
	}
	for _, line := range expectedMatches {
		if !filt.Match(line) {
			t.Errorf("\"%s\" should have matched", line)
		}
	}
}
