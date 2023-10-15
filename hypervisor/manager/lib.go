package manager

import (
	"bufio"
	"os"
)

type bufferedFile struct {
	*os.File
	*bufio.Reader
}

func openBufferedFile(filename string) (*bufferedFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return &bufferedFile{
		File:   file,
		Reader: bufio.NewReader(file),
	}, nil
}

func (r *bufferedFile) Read(p []byte) (int, error) {
	return r.Reader.Read(p)
}
