package filter

import (
	"io"
	"testing"
)

var (
	excludeFilterLines = []string{
		"/.*app-log.*",
		"/etc/fstab",
		"/etc/ssh/ssh_host_.*_key(|[.]pub)$",
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
		"/etc/ssh/ssh_config",
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
		"/etc/ssh/ssh_host_rsa_key",
		"/etc/ssh/ssh_host_rsa_key.pub",
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
	filt.WriteHtml(io.Discard)
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
	filt.WriteHtml(io.Discard)
}
