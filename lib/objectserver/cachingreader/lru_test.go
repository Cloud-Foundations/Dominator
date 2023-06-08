package cachingreader

import (
	"bytes"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
)

var (
	hash0 = hash.Hash{0x00, 0x00, 0xde, 0xed}
	hash1 = hash.Hash{0x01, 0x00, 0xbe, 0xef}
	hash2 = hash.Hash{0x02, 0x00, 0xfe, 0xed}
	hash3 = hash.Hash{0x03, 0x00, 0xbe, 0xef}
	hash4 = hash.Hash{0x04, 0x00, 0xac, 0xdc}

	object0 = &objectType{hash: hash0}
	object1 = &objectType{hash: hash1}
	object2 = &objectType{hash: hash2}
	object3 = &objectType{hash: hash3}
	object4 = &objectType{hash: hash4}
)

func TestReadAndEmptyLRU(t *testing.T) {
	objSrv := &ObjectServer{
		logger: testlogger.New(t),
		objects: map[hash.Hash]*objectType{
			hash0: object0,
			hash1: object1,
			hash2: object2,
			hash3: object3,
		},
	}
	buffer := &bytes.Buffer{}
	buffer.Write(object0.hash[:])
	buffer.Write(object1.hash[:])
	buffer.Write(object2.hash[:])
	if err := objSrv.readLru(buffer); err != nil {
		t.Fatal(err)
	}
	objSrv.linkOrphanedEntries()
	objSrv.rwLock.Lock()
	if objSrv.newest == nil {
		t.Fatal("No newest entry")
	}
	if objSrv.oldest == nil {
		t.Fatal("No oldest entry")
	}
	objSrv.objects[hash4] = object4
	objSrv.addToLruWithLock(object4)
	count := 0
	for object := objSrv.oldest; object != nil; object = objSrv.oldest {
		t.Logf("Removing: %p, %x", object, object.hash)
		objSrv.removeFromLruWithLock(object)
		if object.newer != nil {
			t.Fatal("object.newer != nil")
		}
		if object.older != nil {
			t.Fatal("object.older != nil")
		}
		count++
	}
	if count != 5 {
		t.Fatalf("count: %d != 5", count)
	}
	if objSrv.newest != nil {
		t.Fatal("Have newest entry")
	}
	if objSrv.oldest != nil {
		t.Fatal("Have oldest entry")
	}
	defer func() { recover() }()
	objSrv.removeFromLruWithLock(object2)
	t.Fatal("Duplicate remove did not panic")
}
