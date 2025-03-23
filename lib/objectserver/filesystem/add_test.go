package filesystem

import (
	"os"
	"strings"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
)

type resultType struct {
	error error
	isNew bool
}

func TestAddSameObjectConcurrent(t *testing.T) {
	defer os.Remove("testfile")
	buffer := make([]byte, 3901)
	logger := testlogger.New(t)
	objSrv := &ObjectServer{Params: Params{Logger: logger}}
	numConcurrent := 100
	resultChannel := make(chan resultType, numConcurrent)
	for index := 0; index < numConcurrent; index++ {
		go addOrCompare(objSrv, resultChannel, buffer)
	}
	var newCount uint
	for index := 0; index < numConcurrent; index++ {
		result := <-resultChannel
		if result.error != nil {
			t.Error(result.error)
		}
		if result.isNew {
			newCount++
		}
	}
	if newCount < 1 {
		t.Error("no new objects")
	}
	if newCount > 1 {
		t.Errorf("multiple new objects: %d", newCount)
	}
}

func TestAddWithCollisions(t *testing.T) {
	defer os.Remove("testfile")
	buffer0 := make([]byte, 16384)
	buffer1 := make([]byte, 16384)
	buffer1[12345] = 1
	logger := testlogger.New(t)
	objSrv := &ObjectServer{Params: Params{Logger: logger}}
	isNew, err := objSrv.addOrCompare(hash.Hash{}, buffer0, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Fatal("existing object")
	}
	isNew, err = objSrv.addOrCompare(hash.Hash{}, buffer1, "testfile")
	if err == nil {
		t.Fatal("no error detected for collision")
	}
	if !strings.Contains(err.Error(), "collision detected:") {
		t.Fatal("no collision detected")
	}
	t.Log(err)
}

func addOrCompare(objSrv *ObjectServer, resultChannel chan<- resultType,
	buffer []byte) {
	isNew, err := objSrv.addOrCompare(hash.Hash{}, buffer, "testfile")
	resultChannel <- resultType{err, isNew}
}
