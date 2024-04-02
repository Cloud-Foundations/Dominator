package json

import (
	"encoding/json"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/uncommenter"
)

func readFromFile(filename string, value interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return Read(file, value)
}

func read(reader io.Reader, value interface{}) error {
	decoder := json.NewDecoder(uncommenter.New(reader,
		uncommenter.CommentTypeAll))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	return nil
}
