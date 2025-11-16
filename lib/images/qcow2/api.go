package qcow2

type Header struct {
	Size uint64
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
