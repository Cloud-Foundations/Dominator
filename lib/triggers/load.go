package triggers

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	libjson "github.com/Cloud-Foundations/Dominator/lib/json"
)

func load(filename string) (*Triggers, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Read(file)
}

func decode(jsonData []byte) (*Triggers, error) {
	var trig Triggers
	if err := json.Unmarshal(jsonData, &trig.Triggers); err != nil {
		return nil, errors.New("error decoding triggers " + err.Error())
	}
	return &trig, nil
}

func read(reader io.Reader) (*Triggers, error) {
	var trig Triggers
	if err := libjson.Read(reader, &trig.Triggers); err != nil {
		return nil, errors.New("error decoding triggers " + err.Error())
	}
	return &trig, nil
}
