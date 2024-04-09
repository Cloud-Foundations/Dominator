package decoders

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/uncommenter"
)

type decoderMap map[string]DecoderGenerator

var defaultDecoders = decoderMap{
	".gob":  func(r io.Reader) Decoder { return gob.NewDecoder(r) },
	".json": func(r io.Reader) Decoder { return json.NewDecoder(r) },
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
		reader := uncommenter.New(file, uncommenter.CommentTypeAll)
		return decoderGenerator(reader).Decode(value)
	}
}

func (decoders decoderMap) findAndDecodeFile(basename string,
	value interface{}) error {
	for ext, decoderGenerator := range decoders {
		filename := basename + ext
		if file, err := os.Open(filename); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		} else {
			defer file.Close()
			reader := uncommenter.New(file, uncommenter.CommentTypeAll)
			if err := decoderGenerator(reader).Decode(value); err != nil {
				return fmt.Errorf("%s: %s", filename, err)
			}
			return nil
		}
	}
	return os.ErrNotExist
}
