package mdb

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

var (
	stringType = reflect.TypeOf([]string{})
)

func makeNonzeroMachine(t *testing.T, zeroIndex int) Machine {
	var machine Machine
	machineValue := reflect.ValueOf(&machine).Elem()
	machineType := reflect.TypeOf(machine)
	for index := 0; index < machineValue.NumField(); index++ {
		if index == zeroIndex {
			continue
		}
		fieldValue := machineValue.Field(index)
		fieldKind := fieldValue.Kind()
		fieldName := machineType.Field(index).Name
		switch fieldKind {
		case reflect.Bool:
			fieldValue.SetBool(true)
		case reflect.String:
			fieldValue.SetString(fieldName)
		case reflect.Ptr:
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		case reflect.Map:
			mapValue := reflect.MakeMap(fieldValue.Type())
			fieldValue.Set(mapValue)
			mapValue.SetMapIndex(reflect.ValueOf("key"),
				reflect.ValueOf("value"))
		case reflect.Slice:
			sliceValue := reflect.MakeSlice(stringType, 2, 2)
			fieldValue.Set(sliceValue)
			sliceValue.Index(0).SetString(fieldName)
			sliceValue.Index(1).SetString(strings.ToLower(fieldName))
		default:
			t.Fatalf("Unsupported field type: %s", fieldKind)
		}
	}
	return machine
}

func TestCompare(t *testing.T) {
	left := makeNonzeroMachine(t, -1)
	right := Machine{Hostname: left.Hostname}
	if got := left.Compare(right); got != false {
		t.Errorf("Compare(%v, %v) = %v", left, right, got)
	}
	right = makeNonzeroMachine(t, -1)
	if got := left.Compare(right); got != true {
		t.Errorf("Compare(%v, %v) = %v", left, right, got)
	}
	right.Tags["key"] = "value"
	if got := left.Compare(right); got != true {
		t.Errorf("Compare(%v, %v) = %v", left, right, got)
	}
	right.Tags["key"] = "another value"
	if got := left.Compare(right); got != false {
		t.Errorf("Compare(%v, %v) = %v", left, right, got)
	}
}

func TestCompareAwsMetadata(t *testing.T) {
	var tests = []struct {
		left, right *AwsMetadata
		want        bool
	}{
		{ // No tags, same metadata.
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil},
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil}, true},
		{ // No tags, different instance.
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil},
			&AwsMetadata{"aId-0", "aName-0", "i-1", "r-0", nil}, false},
		{ // No tags, different AccountId.
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil},
			&AwsMetadata{"aId-1", "aName-0", "i-0", "r-0", nil}, false},
		{ // No tags, different AccountName.
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil},
			&AwsMetadata{"aId-0", "aName-1", "i-0", "r-0", nil}, false},
		{ // No tags, different Region.
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-0", nil},
			&AwsMetadata{"aId-0", "aName-0", "i-0", "r-1", nil}, false},
		{ // One tag, the same.
			&AwsMetadata{"aId-0", "aName-0", "i-2", "r-0",
				tags.Tags{"k0": "v0"}},
			&AwsMetadata{"aId-0", "aName-0", "i-2", "r-0",
				tags.Tags{"k0": "v0"}},
			true,
		},
		{ // One tag, different.
			&AwsMetadata{"aId-0", "aName-0", "i-3", "r-0",
				tags.Tags{"k0": "v0"}},
			&AwsMetadata{"aId-0", "aName-0", "i-3", "r-0",
				tags.Tags{"k0": "v1"}},
			false,
		},
		{ // Two tags, the same.
			&AwsMetadata{"aId-0", "aName-0", "i-4", "r-0",
				tags.Tags{"k0": "v0", "k1": "v1"}},
			&AwsMetadata{"aId-0", "aName-0", "i-4", "r-0",
				tags.Tags{"k0": "v0", "k1": "v1"}},
			true,
		},
		{ // Two tags added in a different order, the same
			&AwsMetadata{"aId-0", "aName-0", "i-5", "r-0",
				tags.Tags{"k0": "v0", "k1": "v1"}},
			&AwsMetadata{"aId-0", "aName-0", "i-5", "r-0",
				tags.Tags{"k1": "v1", "k0": "v0"}},
			true,
		},
		{ // Two tags, values swapped.
			&AwsMetadata{"aId-0", "aName-0", "i-6", "r-0",
				tags.Tags{"k0": "v0", "k1": "v1"}},
			&AwsMetadata{"aId-0", "aName-0", "i-6", "r-0",
				tags.Tags{"k0": "v1", "k1": "v0"}},
			false,
		},
	}
	for _, test := range tests {
		if got := compareAwsMetadata(test.left, test.right); got != test.want {
			t.Errorf("Less(%q, %q) = %v", test.left, test.right, got)
		}
	}
}

func TestCompareEachField(t *testing.T) {
	left := makeNonzeroMachine(t, -1)
	for index := 0; index < reflect.TypeOf(left).NumField(); index++ {
		right := makeNonzeroMachine(t, index)
		if got := left.Compare(right); got != false {
			t.Errorf("Compare(%v, %v) = %v", left, right, got)
		}
	}
}
