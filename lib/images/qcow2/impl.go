package qcow2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
)

var (
	magic = []byte("QFI\xfb")
)

func readHeaderFromFile(filename string) (*Header, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, 72)
	nRead, err := file.Read(buffer)
	if err != nil {
		return nil, err
	}
	if nRead < len(buffer) {
		return nil, errors.New("short read")
	}
	var header Header
	if err := Unmarshal(buffer, &header); err != nil {
		return nil, err
	}
	return &header, nil
}

func unmarshal(data []byte, v *Header) error {
	if len(data) < 72 {
		return errors.New("header too short")
	}
	if !bytes.Equal(data[:4], magic) {
		return errors.New("QEMU magic value missing")
	}
	v.Size = binary.BigEndian.Uint64(data[24:32])
	return nil
}
