package qcow2

import (
	"io"
)

type Header struct {
	Size uint64
}

type Peeker interface {
	Peek(n int) ([]byte, error)
}

// PeekHeader will peek into the Peeker and decode a QCOW2 header.
// It returns a *Header on success, else an error.
func PeekHeader(peeker Peeker) (*Header, error) {
	return peekHeader(peeker)
}

// ReadHeader will read a QCOW2 header from an io.Reader.
// It returns a *Header on success, else an error.
func ReadHeader(reader io.Reader) (*Header, error) {
	return readHeader(reader)
}

// ReadHeaderFromFile will read a QCOW2 header from a specified file.
// It returns a *Header on success, else an error.
func ReadHeaderFromFile(filename string) (*Header, error) {
	return readHeaderFromFile(filename)
}

// Unmarshal parses a QCOW2 header from the provided data and stores the result
// in the value pointed to by v. If the data are not a valid QCOW2 header an
// error is returned.
func Unmarshal(data []byte, v *Header) error {
	return unmarshal(data, v)
}
