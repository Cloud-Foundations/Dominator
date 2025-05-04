package hash

import (
	"bytes"
	"crypto/sha512"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

func makeRandomHash() Hash {
	buffer := make([]byte, 1024)
	if nRead, err := rand.Read(buffer); err != nil {
		panic(err)
	} else if nRead != len(buffer) {
		panic(fmt.Sprintf("read: %d, expected: %d", nRead, len(buffer)))
	}
	hasher := sha512.New()
	if _, err := io.Copy(hasher, bytes.NewReader(buffer)); err != nil {
		panic(err)
	}
	var hashVal Hash
	copy(hashVal[:], hasher.Sum(nil))
	return hashVal
}

func TestConvert(t *testing.T) {
	for range 10 {
		hashVal := makeRandomHash()
		text, err := hashVal.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var unmarshaledHash Hash
		if err := unmarshaledHash.UnmarshalText(text); err != nil {
			t.Fatal(err)
		}
		if unmarshaledHash != hashVal {
			t.Errorf("expected: %x, got: %x", hashVal, unmarshaledHash)
		}
	}
}
