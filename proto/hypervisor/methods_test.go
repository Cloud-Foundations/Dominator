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

func makeNonzeroVmInfo(t *testing.T, zeroIndex int) (VmInfo, bool) {
	setAllRequested := true
	var vmInfo VmInfo
	vmInfoValue := reflect.ValueOf(&vmInfo).Elem()
	vmInfoType := reflect.TypeOf(vmInfo)
	for index := 0; index < vmInfoValue.NumField(); index++ {
		if index == zeroIndex {
			continue
		}
		fieldValue := vmInfoValue.Field(index)
		fieldKind := fieldValue.Kind()
		fieldName := vmInfoType.Field(index).Name
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
			switch fieldName {
			case "OwnerGroups", "OwnerUsers", "SecondarySubnetIDs":
				sliceValue := reflect.MakeSlice(stringType, 2, 2)
				fieldValue.Set(sliceValue)
				sliceValue.Index(0).SetString(fieldName)
				sliceValue.Index(1).SetString(strings.ToLower(fieldName))
			case "SecondaryAddresses":
				addresses := []Address{{
					IpAddress:  []byte{1, 2, 3, 4},
					MacAddress: "01:02:03",
				}}
				fieldValue.Set(reflect.ValueOf(addresses))
			case "Volumes":
				volumes := []Volume{{
					Format: 1,
					Size:   2,
					Type:   3,
				}}
				fieldValue.Set(reflect.ValueOf(volumes))
			default:
				t.Fatalf("Unsupported slice field: %s", fieldName)
			}
		case reflect.Struct:
			switch fieldName {
			case "Address":
				address := Address{
					IpAddress:  []byte{1, 2, 3, 4},
					MacAddress: "01:02:03",
				}
				fieldValue.Set(reflect.ValueOf(address))
			case "ChangedStateOn", "CreatedOn", "IdentityExpires":
				fieldValue.Set(reflect.ValueOf(startTime))
			default:
				t.Fatalf("Unsupported struct field: %s", fieldName)
			}
		case reflect.Uint, reflect.Uint64:
			fieldValue.SetUint(1)
		default:
			t.Fatalf("Unsupported field type: %s", fieldKind)
		}
	}
	return vmInfo, setAllRequested
}

func TestCompare(t *testing.T) {
	left, _ := makeNonzeroVmInfo(t, -1)
	right := VmInfo{Hostname: left.Hostname}
	if got := left.Equal(&right); got != false {
		t.Errorf("Equal(%v, %v) = %v", left, right, got)
	}
	right, _ = makeNonzeroVmInfo(t, -1)
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
}

func TestCompareEachField(t *testing.T) {
	left, _ := makeNonzeroVmInfo(t, -1)
	vmInfoType := reflect.TypeOf(left)
	for index := 0; index < vmInfoType.NumField(); index++ {
		right, _ := makeNonzeroVmInfo(t, index)
		if got := left.Equal(&right); got != false {
			t.Errorf("Field: %s not being compared",
				vmInfoType.Field(index).Name)
		}
	}
}
