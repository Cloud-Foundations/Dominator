package decoders

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type decoderMap map[string]DecoderGenerator

var defaultDecoders = decoderMap{
	".gob":  func(r io.Reader) Decoder { return gob.NewDecoder(r) },
	".json": func(r io.Reader) Decoder { return json.NewDecoder(r) },
}

func decodeFile(filename string, value interface{}) error {
	return defaultDecoders.decodeFile(filename, value)
}

func registerDecoder(extension string, decoderGenerator DecoderGenerator) {
	defaultDecoders[extension] = decoderGenerator
}

func (decoders decoderMap) decodeFile(filename string,
	value interface{}) error {
	ext := filepath.Ext(filename)
	if ext == "" {
		return fmt.Errorf("no extension for file: %s", filename)
	}
	decoderGenerator, ok := decoders[filepath.Ext(filename)]
	if !ok {
		return fmt.Errorf("no decoder for .%s extension", ext)
	}
	if file, err := os.Open(filename); err != nil {
		return err
	} else {
		defer file.Close()
		return decoderGenerator(file).Decode(value)
	}
}