package fleetmanager

import (
	"reflect"
	"testing"
)

// makeNonZeroHypervisorData will make a HypervisorData object which is filled
// with non-zero values except the field specified by zeroIndex, which will be
// filled with a zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeNonZeroHypervisorData(t *testing.T, zeroIndex int, base,
	subBase uint) HypervisorData {
	return makeHypervisorData(t, true, zeroIndex, base, subBase)
}

// makeHypervisorData will make a HypervisorData object which is filled with
// non-zero values if nonZeroFill is true else filled with zero values.
// The field specified by invertIndex will be different, either filled with a
// zero value or a non-zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeHypervisorData(t *testing.T, nonZeroFill bool, invertIndex int,
	base, subBase uint) HypervisorData {
	var vmInfo HypervisorData
	vmInfoValue := reflect.ValueOf(&vmInfo).Elem()
	vmInfoType := reflect.TypeOf(vmInfo)
	for index := 0; index < vmInfoValue.NumField(); index++ {
		if nonZeroFill {
			if index == invertIndex {
				continue
			}
		} else {
			if index != invertIndex {
				continue
			}
		}
		fieldValue := vmInfoValue.Field(index)
		fieldKind := fieldValue.Kind()
		fieldName := vmInfoType.Field(index).Name
		switch fieldKind {
		case reflect.Bool:
			fieldValue.SetBool(true)
		case reflect.Int, reflect.Int64:
			fieldValue.SetInt(int64(base) + 1)
		case reflect.String:
			fieldValue.SetString(fieldName)
		case reflect.Ptr:
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		case reflect.Map:
			mapValue := reflect.MakeMap(fieldValue.Type())
			fieldValue.Set(mapValue)
			mapValue.SetMapIndex(reflect.ValueOf("key"),
				reflect.ValueOf(base+1))
		case reflect.Uint, reflect.Uint64:
			fieldValue.SetUint(uint64(base) + 1)
		default:
			t.Fatalf("Unsupported field type: %s", fieldKind)
		}
	}
	return vmInfo
}

// makeZeroHypervisorData will make a HypervisorData object which is filled with
// zero values except the field specified by nonZeroIndex, which will be filled
// with a non-zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeZeroHypervisorData(t *testing.T, nonZeroIndex int, base,
	subBase uint) HypervisorData {
	return makeHypervisorData(t, false, nonZeroIndex, base, subBase)
}

func TestCompare(t *testing.T) {
	left := makeNonZeroHypervisorData(t, -1, 0, 0)
	right := HypervisorData{}
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right = makeNonZeroHypervisorData(t, -1, 0, 0)
	if got := left.Equal(&right); got != true {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
}

func TestCompareEachField(t *testing.T) {
	leftZero := HypervisorData{}
	leftNonZero := makeNonZeroHypervisorData(t, -1, 0, 0)
	vmInfoType := reflect.TypeOf(HypervisorData{})
	for index := 0; index < vmInfoType.NumField(); index++ {
		rightNonZero := makeNonZeroHypervisorData(t, index, 100, 1000)
		if got := leftNonZero.Equal(&rightNonZero); got != false {
			t.Errorf("Field: %s with zero data not being compared",
				vmInfoType.Field(index).Name)
			continue
		}
		rightZero := makeZeroHypervisorData(t, index, 100, 1000)
		if got := leftZero.Equal(&rightZero); got != false {
			t.Errorf("Field: %s with non-zero data not being compared",
				vmInfoType.Field(index).Name)
			continue
		}
	}
}
