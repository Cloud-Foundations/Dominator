package uncommenter

import (
	"io"
)

const (
	CommentTypeHash       = 1 << iota // "#"
	CommentTypeSlashSlash             // "//"
	CommentTypeBang                   // "!"

	CommentTypeAll = 0xffffffffffffffff
)

type bufferedReader interface {
	Read(p []byte) (int, error)
	ReadByte() (byte, error)
}

type uncommenter struct {
	commentTypes      uint64
	error             error
	reader            bufferedReader
	waitingForNewline bool
}

// New will return a wrapped reader, filtering out comment lines.
// Comment lines may begin with arbitrary whitespace followed by any of the
// specified commentTypes, until the next newline.
func New(reader io.Reader, commentTypes uint64) io.Reader {
	return newUncommenter(reader, commentTypes)
}

func (u *uncommenter) Read(p []byte) (int, error) {
	return u.read(p)
}
