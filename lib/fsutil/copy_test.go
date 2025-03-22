package fsutil

import (
	"os"
	"testing"
)

type fakeReader struct{}

func TestCopyToFileExclusive(t *testing.T) {
	numConcurrent := 100
	errorChannel := make(chan error, numConcurrent)
	for index := 0; index < numConcurrent; index++ {
		go writeTestfile(t, errorChannel)
	}
	var numCreated, numAlreadyExists int
	for index := 0; index < numConcurrent; index++ {
		if err := <-errorChannel; err == nil {
			numCreated++
		} else if os.IsExist(err) {
			numAlreadyExists++
		} else {
			t.Error(err)
		}
	}
	if numCreated != 1 {
		t.Errorf("numCreated: %d != 1", numCreated)
	}
	if numAlreadyExists != numConcurrent-1 {
		t.Errorf("numAlreadyExists: %d != %d",
			numAlreadyExists, numConcurrent-1)
	}
	os.Remove("testfile")
}

func writeTestfile(t *testing.T, errorChannel chan<- error) {
	errorChannel <- CopyToFileExclusive("testfile", PublicFilePerms,
		&fakeReader{}, 3900)
}

func (r *fakeReader) Read(p []byte) (int, error) {
	return len(p), nil
}
