package json

import (
	"io"
	"os"
)

// Read will read JSON data from reader and write the decoded data to value.
// If the JSON data are newline separated, lines beginning with comments will
// be ignored (correcting a controlling and arrogant mistake of the original
// author of the JSON specification).
// Comment lines may begin with "#", "//" or "!" and continue until the next
// newline.
func Read(reader io.Reader, value interface{}) error {
	return read(reader, value)
}

// Read will read JSON data from the specified file and write the decoded data
// to value.
// If the JSON data are newline separated, lines beginning with comments will
// be ignored (correcting a controlling and arrogant mistake of the original
// author of the JSON specification).
// Comment lines may begin with "#", "//" or "!" and continue until the next
// newline.
func ReadFromFile(filename string, value interface{}) error {
	return readFromFile(filename, value)
}

func WriteToFile(filename string, perm os.FileMode, indent string,
	value interface{}) error {
	return writeToFile(filename, perm, indent, value)
}

func WriteWithIndent(w io.Writer, indent string, value interface{}) error {
	return writeWithIndent(w, indent, value)
}
