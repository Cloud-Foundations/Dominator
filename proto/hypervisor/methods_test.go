package hypervisor

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	startTime  = time.Now()
	stringType = reflect.TypeOf([]string{})
)

// makeNonZeroVmInfo will make a VmInfo object which is filled with non-zero
// values except the field specified by zeroIndex, which will be filled with a
// zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeNonZeroVmInfo(t *testing.T, zeroIndex int, base, subBase uint) VmInfo {
	return makeVmInfo(t, true, zeroIndex, base, subBase)
}

// makeVmInfo will make a VmInfo object which is filled with non-zero values if
// nonZeroFill is true else filled with zero values.
// The field specified by invertIndex will be different, either filled with a
// zero value or a non-zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeVmInfo(t *testing.T, nonZeroFill bool, invertIndex int,
	base, subBase uint) VmInfo {
	var vmInfo VmInfo
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
				reflect.ValueOf("value"))
		case reflect.Slice:
			switch fieldName {
			case "OwnerGroups", "OwnerUsers", "SecondarySubnetIDs":
				sliceValue := reflect.MakeSlice(stringType, 2, 2)
				fieldValue.Set(sliceValue)
				sliceValue.Index(0).SetString(fieldName)
				sliceValue.Index(1).SetString(strings.ToLower(fieldName))
			case "SecondaryAddresses":
				addresses := []Address{{
					[]byte{1, 2, 3, 4},
					"01:02:03",
				}}
				fieldValue.Set(reflect.ValueOf(addresses))
			case "Volumes":
				volumes := []Volume{{
					VolumeFormat(base) + 1,
					VolumeInterface(base) + 2,
					uint64(base) + 3,
					map[string]uint64{
						"":    uint64(subBase) + 4,
						"foo": uint64(subBase) + 5,
					},
					VolumeType(base) + 6,
				}}
				fieldValue.Set(reflect.ValueOf(volumes))
			default:
				t.Fatalf("Unsupported slice field: %s", fieldName)
			}
		case reflect.Struct:
			switch fieldName {
			case "Address":
				address := Address{
					[]byte{1, 2, 3, 4},
					"01:02:03",
				}
				fieldValue.Set(reflect.ValueOf(address))
			case "ChangedStateOn", "CreatedOn", "IdentityExpires":
				fieldValue.Set(reflect.ValueOf(startTime))
			default:
				t.Fatalf("Unsupported struct field: %s", fieldName)
			}
		case reflect.Uint, reflect.Uint64:
			fieldValue.SetUint(uint64(base) + 1)
		default:
			t.Fatalf("Unsupported field type: %s", fieldKind)
		}
	}
	return vmInfo
}

// makeZeroVmInfo will make a VmInfo object which is filled with zero values
// except the field specified by nonZeroIndex, which will be filled with a
// non-zero value.
// When filling fields, the value of base is added to some internal constants.
// When filling sub-fields, the value of subBase is added to some internal
// constants.
func makeZeroVmInfo(t *testing.T, nonZeroIndex int, base, subBase uint) VmInfo {
	return makeVmInfo(t, false, nonZeroIndex, base, subBase)
}

func TestCompare(t *testing.T) {
	left := makeNonZeroVmInfo(t, -1, 0, 0)
	right := VmInfo{Hostname: left.Hostname}
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right = makeNonZeroVmInfo(t, -1, 0, 0)
	if got := left.Equal(&right); got != true {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right.Tags["key"] = "value"
	if got := left.Equal(&right); got != true {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right.Tags["key"] = "another value"
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right = makeNonZeroVmInfo(t, -1, 100, 1000)
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	left = makeNonZeroVmInfo(t, -1, 100, 500)
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
}

func TestCompareEachField(t *testing.T) {
	leftZero := VmInfo{}
	leftNonZero := makeNonZeroVmInfo(t, -1, 0, 0)
	vmInfoType := reflect.TypeOf(VmInfo{})
	for index := 0; index < vmInfoType.NumField(); index++ {
		rightNonZero := makeNonZeroVmInfo(t, index, 100, 1000)
		if got := leftNonZero.Equal(&rightNonZero); got != false {
			t.Errorf("Field: %s with zero data not being compared",
				vmInfoType.Field(index).Name)
			continue
		}
		rightZero := makeZeroVmInfo(t, index, 100, 1000)
		if got := leftZero.Equal(&rightZero); got != false {
			t.Errorf("Field: %s with non-zero data not being compared",
				vmInfoType.Field(index).Name)
			continue
		}
	}
}
