package main

import (
	"reflect"
	"testing"
)

func TestScanTypes(t *testing.T) {
	infos, err := scanTypes("testdata/proto",
		[]string{"GetVmInfoRequest", "GetVmInfoResponse", "ListVMsRequest"})
	if err != nil {
		t.Fatal(err)
	}
	want := []typeInfo{
		{"Address", []fieldInfo{
			{"IpAddress", "net.IP", 1},
			{"MacAddress", "string", 2},
		}},
		{"GetVmInfoRequest", []fieldInfo{{"IpAddress", "net.IP", 1}}},
		{"GetVmInfoResponse", []fieldInfo{{"VmInfo", "VmInfo", 1}}},
		{"ListVMsRequest", []fieldInfo{
			{"OwnerUsers", "[]string", 1},
			{"Sort", "bool", 3},
			{"Tags", "tags.Tags", 2},
		}},
		{"Standalone", []fieldInfo{{"Name", "string", 1}}},
		{"VmInfo", []fieldInfo{
			{"Address", "Address", 1},
			{"Hostname", "string", 5},
			{"Secondary", "[]*Address", 2},
			{"State", "State", 3},
			{"Volumes", "map[string]Address", 4},
			{"First", "uint64", 6},
			{"Next", "uint64", 7},
		}},
	}
	if !reflect.DeepEqual(infos, want) {
		t.Errorf("got:  %v\nwant: %v", infos, want)
	}
}

func TestScanTypesAutoNumberSkipsExplicit(t *testing.T) {
	dir := writeSource(t, `package p
type Request struct {
	A string `+"`proto:\"1\"`"+`
	B string
	C string `+"`proto:\"2\"`"+`
}
`)
	infos, err := scanTypes(dir, []string{"Request"})
	if err != nil {
		t.Fatal(err)
	}
	want := []fieldInfo{{"A", "string", 1}, {"B", "string", 3},
		{"C", "string", 2}}
	if !reflect.DeepEqual(infos[0].Fields, want) {
		t.Errorf("got %v, want %v", infos[0].Fields, want)
	}
}

func TestScanTypesGroupedAnnotation(t *testing.T) {
	dir := writeSource(t, `package p
type (
	// @grpc
	Tagged struct{ A string }
	Plain  struct{ B string }
)
`)
	infos, err := scanTypes(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Name != "Tagged" {
		t.Errorf("got %v, want only Tagged", infos)
	}
}

func TestScanTypesErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		root   string
	}{
		{"missing root", "package p\n", "Request"},
		{"root not a struct", "package p\ntype Request uint\n", "Request"},
		{"duplicate number", `package p
type Request struct {
	A string ` + "`proto:\"1\"`" + `
	B string ` + "`proto:\"1\"`" + `
}
`, "Request"},
		{"invalid number", `package p
type Request struct {
	A string ` + "`proto:\"zero\"`" + `
}
`, "Request"},
		{"bad annotation on type", `package p
// @grpc Name
type Other struct{ A string }
`, "Other"},
		{"embedded field", "package p\ntype Request struct{ Base }\n",
			"Request"},
		{"func field", "package p\ntype Request struct{ F func() }\n",
			"Request"},
		{"error in nested type", `package p
type Request struct{ Inner Inner }
type Inner struct{ F chan int }
`, "Request"},
	}
	for _, test := range tests {
		dir := writeSource(t, test.source)
		if _, err := scanTypes(dir, []string{test.root}); err == nil {
			t.Errorf("%s: no error", test.name)
		}
	}
}

func TestScanNamedTypes(t *testing.T) {
	named, err := scanNamedTypes("testdata/proto")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Blob":   "[]byte",
		"Labels": "tags.Tags",
		"State":  "uint",
	}
	if !reflect.DeepEqual(named, want) {
		t.Errorf("got %v, want %v", named, want)
	}
	if _, ok := named["VmInfo"]; ok {
		t.Error("struct VmInfo reported as a named non-struct type")
	}
}
