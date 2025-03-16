package pathregexp

import (
	"testing"
)

type expressionType struct {
	expression      string
	expectOptimised bool
}

type regexpList []Regexp

var (
	expressions = []expressionType{
		{".*/__pycache__/.*", true},
		{"/.*app-log.*", true},
		{"/etc/fstab", true},
		{"/etc/ssh/ssh_host_.*_key(|[.]pub)$", true},
		{"/file.log", false},
		{"/foo(|.*)$", false},
		{"/tmp(|/.*)$", true},
	}
)

func TestMatch(t *testing.T) {
	reList := make(regexpList, 0, len(expressions))
	for _, expression := range expressions {
		re, err := Compile(expression.expression)
		if err != nil {
			t.Error(err)
		}
		if IsOptimised(re) != expression.expectOptimised {
			t.Errorf("expression: \"%s\": IsOptimised=%v, expected: %v\n",
				expression.expression,
				IsOptimised(re), expression.expectOptimised)
		}
		reList = append(reList, re)
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
		if reList.match(line) {
			t.Errorf("\"%s\" should not have matched", line)
		}
	}
	expectedMatches := []string{
		"/.myapp-log.err",
		"/.myapp-logout",
		"/__pycache__/dir",
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
		"/usr/lib/__pycache__/dir",
		"/usr/lib/__pycache__/dir/file",
	}
	for _, line := range expectedMatches {
		if !reList.match(line) {
			t.Errorf("\"%s\" should have matched", line)
		}
	}
}

func (list regexpList) match(s string) bool {
	for _, re := range list {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
