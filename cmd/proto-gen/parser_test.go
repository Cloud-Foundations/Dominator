package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeSource writes source to a Go file in a new temporary directory and
// returns the directory.
func writeSource(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "file.go"), []byte(source), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindMethods(t *testing.T) {
	methods, err := findMethods("testdata/rpcd")
	if err != nil {
		t.Fatal(err)
	}
	want := []methodInfo{
		{"GetVmInfo", "GetVmInfoRequest", "GetVmInfoResponse", false},
		{"ListVMs", "ListVMsRequest", "ListVMsResponse", true},
	}
	if !reflect.DeepEqual(methods, want) {
		t.Errorf("got:  %v\nwant: %v", methods, want)
	}
}

func TestFindMethodsErrors(t *testing.T) {
	tests := map[string]string{
		"streaming": `package p
// @grpc
func (t *T) Stream(conn *srpc.Conn) error { return nil }
`,
		"unknown argument": `package p
// @grpc ListVms
func (t *T) M(c *srpc.Conn, r Req, p *Rep) error { return nil }
`,
		"two arguments": `package p
// @grpc no-rest extra
func (t *T) M(c *srpc.Conn, r Req, p *Rep) error { return nil }
`,
		"unsupported request type": `package p
// @grpc
func (t *T) A(c *srpc.Conn, r []Req, p *Rep) error { return nil }
`,
	}
	for name, source := range tests {
		if _, err := findMethods(writeSource(t, source)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestFindMethodsIgnoresPlainFunctions(t *testing.T) {
	methods, err := findMethods(writeSource(t, `package p
// @grpc
func notAMethod() {}
`))
	if err != nil || len(methods) != 0 {
		t.Errorf("got %v, %v", methods, err)
	}
}
