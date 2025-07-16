package topology

import (
	"net"
	"reflect"
	"testing"

	installer_proto "github.com/Cloud-Foundations/Dominator/proto/installer"
)

var (
	ipAddrOne = net.IP{1, 2, 3, 4}
	ipAddrTwo = net.IP{5, 6, 7, 8}
)

func testNonzero(t *testing.T, valueValue reflect.Value, equalTest func(),
	notEqualTest func()) {
	valueType := valueValue.Type()
	for index := 0; index < valueValue.NumField(); index++ {
		fieldValue := valueValue.Field(index)
		if !fieldValue.CanSet() {
			continue
		}
		fieldKind := fieldValue.Kind()
		switch fieldKind {
		case reflect.Bool:
			equalTest()
			fieldValue.SetBool(true)
			notEqualTest()
			fieldValue.SetBool(false)
			equalTest()
		case reflect.Map:
			equalTest()
			mapValue := reflect.MakeMap(fieldValue.Type())
			fieldValue.Set(mapValue)
			mapValue.SetMapIndex(reflect.ValueOf("key"),
				reflect.ValueOf("value"))
			notEqualTest()
			fieldValue.Set(reflect.MakeMap(fieldValue.Type()))
			equalTest()
		case reflect.Ptr:
			testNonzero(t, reflect.Indirect(fieldValue), equalTest,
				notEqualTest)
		case reflect.Slice:
			for index := 0; index < fieldValue.Len(); index++ {
				testNonzero(t, fieldValue.Index(index), equalTest, notEqualTest)
			}
		case reflect.String:
			equalTest()
			fieldValue.SetString(valueType.Field(index).Name)
			notEqualTest()
			fieldValue.SetString("")
			equalTest()
		case reflect.Struct:
			testNonzero(t, fieldValue, equalTest, notEqualTest)
		case reflect.Uint:
			equalTest()
			fieldValue.SetUint(1)
			notEqualTest()
			fieldValue.SetUint(0)
			equalTest()
		default:
			t.Fatalf("Unsupported field type: %s", fieldKind)
		}
	}
}

func TestCompareAllDirectoryFields(t *testing.T) {
	left := &Directory{
		InstallConfig: &InstallConfig{
			StorageLayout: &installer_proto.StorageLayout{},
		},
	}
	right := &Directory{
		InstallConfig: &InstallConfig{
			StorageLayout: &installer_proto.StorageLayout{},
		},
	}
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	testNonzero(t, reflect.ValueOf(right).Elem(),
		func() { // Function to check for equality.
			if got := left.equal(right); got != true {
				t.Errorf("equal(%v, %v) = %v", left, right, got)
			}
		},
		func() { // Function to check for non-equality.
			if got := left.equal(right); got != false {
				t.Errorf("equal(%v, %v) = %v", left, right, got)
			}
		})
}

func TestCompareAllSubnetFields(t *testing.T) {
	left := &Subnet{}
	right := &Subnet{}
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	testNonzero(t, reflect.ValueOf(right).Elem(),
		func() { // Function to check for equality.
			if got := left.equal(right); got != true {
				t.Errorf("equal(%v, %v) = %v", left, right, got)
			}
		},
		func() { // Function to check for non-equality.
			if got := left.equal(right); got != false {
				t.Errorf("equal(%v, %v) = %v", left, right, got)
			}
		})
}

func TestCompareFirstAuto(t *testing.T) {
	left := &Subnet{}
	right := &Subnet{}
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	left.FirstAutoIP = ipAddrOne
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	right.FirstAutoIP = ipAddrOne
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	right.FirstAutoIP = ipAddrTwo
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	left.FirstAutoIP = nil
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
}

func TestCompareLastAuto(t *testing.T) {
	left := &Subnet{}
	right := &Subnet{}
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	left.LastAutoIP = ipAddrOne
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	right.LastAutoIP = ipAddrOne
	if got := left.equal(right); got != true {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	right.LastAutoIP = ipAddrTwo
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
	left.LastAutoIP = nil
	if got := left.equal(right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
}

func TestCompareVariablesEqual(t *testing.T) {
	left := map[string]string{
		"key0": "value0",
		"key1": "value1",
	}
	right := map[string]string{
		"key0": "value0",
		"key1": "value1",
	}
	if got := compareMaps(left, right); got != true {
		t.Errorf("different(%v, %v) = %v", left, right, got)
	}
}

func TestCompareVariablesNotEqual(t *testing.T) {
	left := map[string]string{
		"key0": "value0",
		"key1": "value1",
	}
	right := map[string]string{
		"key0": "value0",
		"key1": "value2",
	}
	if got := compareMaps(left, right); got != false {
		t.Errorf("equal(%v, %v) = %v", left, right, got)
	}
}
