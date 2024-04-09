package uncommenter

import (
	"bufio"
	"io"
)

func newUncommenter(reader io.Reader, commentTypes uint64) io.Reader {
	if commentTypes == 0 {
		return reader
	}
	if bReader, ok := reader.(bufferedReader); ok {
		return &uncommenter{
			commentTypes: commentTypes,
			reader:       bReader,
		}
	}
	return &uncommenter{
		commentTypes: commentTypes,
		reader:       bufio.NewReader(reader),
	}
}

func readUntilNewline(reader bufferedReader) error {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if b == '\n' {
			return nil
		}
	}
}

func (u *uncommenter) read(p []byte) (int, error) {
	if len(p) < 1 {
		return 0, u.error
	}
	if len(p) < 2 {
		return u.reader.Read(p)
	}
	for {
		if n, err := u.readOnce(p); n > 0 || err != nil {
			return n, err
		}
	}
}

func (u *uncommenter) readOnce(p []byte) (int, error) {
	if u.error != nil {
		return 0, u.error
	}
	if u.waitingForNewline {
		for count := range p {
			b, err := u.reader.ReadByte()
			if err != nil {
				u.error = err
				return count, err
			}
			p[count] = b
			if b == '\n' {
				u.waitingForNewline = false
				return count + 1, nil
			}
			u.waitingForNewline = true
		}
		return len(p), nil
	}
	// Start of file/line: read until first non-whitespace.
	var nRead int
	for nRead < len(p) {
		p[nRead], u.error = u.reader.ReadByte()
		if u.error != nil {
			return nRead, u.error
		}
		nRead++
		if p[nRead-1] == ' ' {
			continue
		}
		if p[nRead-1] == '\t' {
			continue
		}
		break
	}
	if p[nRead-1] == '#' && u.commentTypes&CommentTypeHash != 0 {
		u.error = readUntilNewline(u.reader)
		return 0, u.error
	}
	if p[nRead-1] == '/' && u.commentTypes&CommentTypeSlashSlash != 0 &&
		nRead < len(p) {
		p[nRead], u.error = u.reader.ReadByte()
		if u.error != nil {
			return nRead - 1, u.error
		}
		nRead++
		if p[nRead-1] == '/' {
			u.error = readUntilNewline(u.reader)
			return 0, u.error
		}
	}
	if p[nRead-1] == '!' && u.commentTypes&CommentTypeBang != 0 {
		u.error = readUntilNewline(u.reader)
		return 0, u.error
	}
	u.waitingForNewline = true
	return nRead, nil
}
