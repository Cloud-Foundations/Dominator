package json

import (
	"bytes"
	"encoding/json"
	"testing"
)

var (
	badJson = []byte(`{
    "Key": "value",
}
`)

	commentedJson = []byte(`{
    "Key": "value"
# Wash away that original sin.
}
`)

	goodJson = []byte(`{
    "Key": "value"
}
`)
)

type jsonDataType struct {
	Key string
}

func TestBad(t *testing.T) {
	var data jsonDataType
	if err := Read(bytes.NewBuffer(badJson), &data); err == nil {
		t.Errorf("No failure trying to read bad JSON")
	}
}

func TestCommentedUnfiltered(t *testing.T) {
	var data jsonDataType
	if err := json.Unmarshal(commentedJson, &data); err == nil {
		t.Errorf("No failure trying to read unfiltered commented JSON")
	}
}

func TestCommentedFiltered(t *testing.T) {
	var data jsonDataType
	if err := Read(bytes.NewBuffer(commentedJson), &data); err != nil {
		t.Errorf("Failure trying to read filtered commented JSON")
	}
	if data.Key != "value" {
		t.Errorf("Key: expected: \"value\", got: \"%s\"", data.Key)
	}
}

func TestGoodFiltered(t *testing.T) {
	var data jsonDataType
	if err := Read(bytes.NewBuffer(goodJson), &data); err != nil {
		t.Errorf("Failure trying to read good JSON")
	}
	if data.Key != "value" {
		t.Errorf("Key: expected: \"value\", got: \"%s\"", data.Key)
	}
}
