package verstr

import (
	"testing"
)

func TestLess(t *testing.T) {
	var tests = []struct {
		left, right string
		want        bool
	}{
		{"", "", false},
		{"", "empty", true},
		{"empty", "", false},
		{"file.0.ext", "file.1.ext", true},
		{"file.1.ext", "file.0.ext", false},
		{"file.1.ext", "file.10.ext", true},
		{"file.10.ext", "file.1.ext", false},
		{"file.9.ext", "file.10.ext", true},
		{"file.10.ext", "file.9.ext", false},
		{"name.1.rc1", "name.1.rc10", true},
		{"name.1.rc10", "name.1.rc1", false},
		{"name.1.rc9", "name.1.rc10", true},
		{"name.1.rc10", "name.1.rc9", false},
		{"os-v0", "os-v1", true},
		{"os-v1", "os-v0", false},
		{"os-v1", "os-v10", true},
		{"os-v10", "os-v1", false},
		{"os-v9", "os-v10", true},
		{"os-v10", "os-v9", false},
		{"sparse", "sparse.0", true},
		{"sparse.0", "sparse", false},
		{"token_01", "token_2", true},
		{"token_2", "token_10", true},
	}
	for _, test := range tests {
		if got := Less(test.left, test.right); got != test.want {
			t.Errorf("Less(%q, %q) = %v", test.left, test.right, got)
		}
	}
}
