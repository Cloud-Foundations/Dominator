package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "Rewrite the golden files")

// testConfig is the configuration of the golden file.
var testConfig = protoConfig{
	GoPackage: "github.com/Cloud-Foundations/Dominator/proto/hypervisor/grpc",
	Package:   "hypervisor",
	Service:   "Hypervisor",
}

// scanTestdata returns the scan of the testdata packages.
func scanTestdata(t *testing.T) ([]methodInfo, []typeInfo,
	map[string]string) {
	t.Helper()
	methods, err := findMethods("testdata/rpcd")
	if err != nil {
		t.Fatal(err)
	}
	infos, err := scanTypes("testdata/proto", rootTypeNames(methods))
	if err != nil {
		t.Fatal(err)
	}
	named, err := scanNamedTypes("testdata/proto")
	if err != nil {
		t.Fatal(err)
	}
	return methods, infos, named
}

func TestGenerateProtoGolden(t *testing.T) {
	methods, infos, named := scanTestdata(t)
	got, err := generateProto(testConfig, methods, infos, named)
	if err != nil {
		t.Fatal(err)
	}
	const goldenFile = "testdata/expected.proto"
	if *update {
		if err := os.WriteFile(goldenFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("generated proto differs from %s (use -update):\n%s",
			goldenFile, got)
	}
}

func TestGenerateProtoWithoutMethods(t *testing.T) {
	_, infos, named := scanTestdata(t)
	got, err := generateProto(testConfig, nil, infos, named)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "service ") {
		t.Errorf("service written without methods:\n%s", got)
	}
	if !strings.HasSuffix(got, "}\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("file does not end with one newline:\n%s", got)
	}
}

func TestProtoType(t *testing.T) {
	messages := map[string]struct{}{"Address": {}}
	named := map[string]string{
		"A":     "map[A]A",
		"Alias": "Address",
		"B":     "uint8",
		"Blob":  "[]byte",
		"Level": "State",
		"Loop":  "Loop",
		"Name":  "string",
		"P":     "Q",
		"Q":     "[]P",
		"State": "uint",
		"X":     "map[string]struct{...}",
	}
	tests := []struct {
		goType  string
		want    string // The proto type, or empty if there should be an error.
		wantErr string // Part of the error message.
	}{
		{"string", "string", ""},
		{"bool", "bool", ""},
		{"int", "int64", ""},
		{"int8", "int32", ""},
		{"int16", "int32", ""},
		{"byte", "uint32", ""},
		{"uint16", "uint32", ""},
		{"rune", "int32", ""},
		{"int64", "int64", ""},
		{"uint", "uint64", ""},
		{"float32", "float", ""},
		{"float64", "double", ""},
		{"complex64", "", "cannot map type complex64"},
		{"uintptr", "", "cannot map type uintptr"},
		{"error", "", "cannot map type error"},
		{"any", "", "cannot map type any"},
		{"[]time.Time", "repeated google.protobuf.Timestamp", ""},
		{"map[string]time.Duration", "map<string, google.protobuf.Duration>",
			""},
		{"map[time.Time]string", "", "map[time.Time]string"},
		{"net.IP", "string", ""},
		{"net.HardwareAddr", "string", ""},
		{"time.Time", "google.protobuf.Timestamp", ""},
		{"time.Duration", "google.protobuf.Duration", ""},
		{"tags.Tags", "map<string, string>", ""},
		{"Address", "Address", ""},
		{"*Address", "Address", ""},
		{"[]*Address", "repeated Address", ""},
		{"[]string", "repeated string", ""},
		{"[]byte", "bytes", ""},
		{"[]uint8", "bytes", ""},
		{"[][]byte", "repeated bytes", ""},
		{"map[string]Address", "map<string, Address>", ""},
		{"map[string][]byte", "map<string, bytes>", ""},
		{"map[net.IP]string", "map<string, string>", ""},
		{"map[State]string", "map<uint64, string>", ""},
		{"map[bool]string", "map<bool, string>", ""},
		{"map[int32]string", "map<int32, string>", ""},
		{"map[int64]string", "map<int64, string>", ""},
		{"map[uint64]string", "map<uint64, string>", ""},
		{"map[uint32]string", "map<uint32, string>", ""},
		{"map[uint8]string", "map<uint32, string>", ""},
		{"map[Blob]string", "", "map[Blob]string"},
		{"Level", "uint64", ""},
		{"Name", "string", ""},
		{"[]State", "repeated uint64", ""},
		{"Alias", "Address", ""},
		{"Blob", "bytes", ""},
		{"[]B", "repeated uint32", ""},
		{"map[State][]State", "", "map[State][]State"},
		{"map[uint64][]string", "", "map[uint64][]string"},
		{"[]tags.Tags", "", "[]tags.Tags"},
		{"[4]byte", "", "array type [4]byte"},
		{"map[float64]string", "", "map[float64]string"},
		{"map[string]map[string]string", "", "map[string]map[string]string"},
		{"Missing", "", "cannot map type Missing"},
		{"Loop", "", "recursive type Loop"},
		{"P", "", "recursive type P"},
		{"X", "", "type X: cannot map type map[string]struct{...} to a proto type"},
		{"os.File", "", "type os.File"},
	}
	for _, test := range tests {
		mapper := newTypeMapper(messages, named)
		got, err := mapper.protoTypeOf(test.goType)
		if test.want == "" {
			if err == nil {
				t.Errorf("%s: got %q, want an error", test.goType, got)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("%s: error %q does not contain %q", test.goType, err,
					test.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %s", test.goType, err)
		} else if got != test.want {
			t.Errorf("%s: got %q, want %q", test.goType, got, test.want)
		}
	}
}

func TestTypeMapperIsReusableAfterAFailure(t *testing.T) {
	named := map[string]string{"Loop": "Loop", "State": "uint"}
	mapper := newTypeMapper(nil, named)
	for _, goType := range []string{"State", "Loop", "State", "[]State",
		"map[State]State", "Loop", "State"} {
		got, err := mapper.protoTypeOf(goType)
		if goType == "Loop" {
			if err == nil {
				t.Errorf("%s: got %q, want an error", goType, got)
			}
		} else if err != nil {
			t.Errorf("%s after earlier lookups: %s", goType, err)
		}
	}
}

func TestGenerateProtoImportsTheTimeTypes(t *testing.T) {
	dir := writeSource(t, `package p
import "time"
type Stamp time.Time
type Request struct {
	Created time.Time
	Expires time.Time
	Timeout time.Duration
	Name    string
}
type Pointer struct {
	When *time.Time
}
type Named struct {
	When Stamp
}
type Plain struct {
	Name string
}
`)
	infos, err := scanTypes(dir, []string{"Request"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := generateProto(testConfig, nil, infos, nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = `package hypervisor;

import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";

option go_package =`
	if !strings.Contains(got, want) {
		t.Errorf("imports missing or out of order:\n%s", got)
	}
	if n := strings.Count(got, "timestamp.proto"); n != 1 {
		t.Errorf("timestamp.proto imported %d times, want once", n)
	}
	for _, root := range []string{"Pointer", "Named"} {
		infos, err = scanTypes(dir, []string{root})
		if err != nil {
			t.Fatal(err)
		}
		named, err := scanNamedTypes(dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err = generateProto(testConfig, nil, infos, named)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `import "google/protobuf/timestamp.proto";`) {
			t.Errorf("%s: timestamp import missing:\n%s", root, got)
		}
	}
	infos, err = scanTypes(dir, []string{"Plain"})
	if err != nil {
		t.Fatal(err)
	}
	got, err = generateProto(testConfig, nil, infos, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "import ") {
		t.Errorf("imports written for a message without time fields:\n%s", got)
	}
}

func TestGenerateProtoRejectsBadNames(t *testing.T) {
	_, infos, named := scanTestdata(t)
	tests := []struct {
		name   string
		config protoConfig
	}{
		{"empty package", protoConfig{GoPackage: "p", Service: "S"}},
		{"package with a space", protoConfig{GoPackage: "p",
			Package: "a b", Service: "S"}},
		{"package with an empty part", protoConfig{GoPackage: "p",
			Package: "a..b", Service: "S"}},
		{"package starting with a digit", protoConfig{GoPackage: "p",
			Package: "1a", Service: "S"}},
		{"empty service", protoConfig{GoPackage: "p", Package: "a"}},
		{"service with a dot", protoConfig{GoPackage: "p", Package: "a",
			Service: "S.T"}},
	}
	for _, test := range tests {
		if _, err := generateProto(test.config, nil, infos, named); err == nil {
			t.Errorf("%s: no error", test.name)
		}
	}
	config := protoConfig{GoPackage: "p", Package: "a.b.c", Service: "S_1"}
	if _, err := generateProto(config, nil, infos, named); err != nil {
		t.Errorf("dotted package: %s", err)
	}
}

func TestGenerateProtoRejectsUnusableConfigs(t *testing.T) {
	methods, infos, named := scanTestdata(t)
	good := protoConfig{GoPackage: "p", Package: "a", Service: "S"}
	if _, err := generateProto(protoConfig{Package: "a", Service: "S"}, methods,
		infos, named); err == nil {
		t.Error("empty Go package: no error")
	}
	clash := protoConfig{GoPackage: "p", Package: "a", Service: "Address"}
	if _, err := generateProto(clash, methods, infos, named); err == nil {
		t.Error("service named like a message: no error")
	}
	if _, err := generateProto(clash, nil, infos, named); err != nil {
		t.Errorf("service name unused without methods: %s", err)
	}
	if _, err := generateProto(good, nil, nil, named); err == nil {
		t.Error("nothing to generate: no error")
	}
	twice := []methodInfo{
		{Name: "Get", RequestType: "Address", ResponseType: "Address"},
		{Name: "Get", RequestType: "VmInfo", ResponseType: "VmInfo"},
	}
	if _, err := generateProto(good, twice, infos, named); err == nil {
		t.Error("duplicate method name: no error")
	}
	one := []methodInfo{{Name: "Get", RequestType: "Address",
		ResponseType: "Address"}}
	got, err := generateProto(good, one, infos, named)
	if err != nil {
		t.Fatal(err)
	}
	if want := "rpc Get(Address) returns (Address);"; !strings.Contains(got,
		want) {
		t.Errorf("one method: output does not contain %q:\n%s", want, got)
	}
}

func TestGenerateProtoNamesTheUnmappableField(t *testing.T) {
	dir := writeSource(t, `package p
type Request struct {
	Name string
	Level Missing
}
`)
	infos, err := scanTypes(dir, []string{"Request"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = generateProto(testConfig, nil, infos, nil)
	if err == nil {
		t.Fatal("no error for an unmappable field")
	}
	if want := "message Request field Level"; !strings.Contains(err.Error(),
		want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
}
