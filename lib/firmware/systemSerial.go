package firmware

import (
	"os"
	"strings"
)

const (
	boardSerialFile   = "/sys/class/dmi/id/board_serial"
	productSerialFile = "/sys/class/dmi/id/product_serial"

	uuidLength = 16
)

func extractSerialNumber(input string) string {
	serial := strings.TrimSpace(input)
	// Ignore some common bogus serial numbers.
	switch serial {
	case "0123456789":
		serial = ""
	case "System Serial Number":
		serial = ""
	case "To be filled by O.E.M.":
		serial = ""
	}
	return serial
}

func readSerialFile(filename string) string {
	if file, err := os.Open(filename); err != nil {
		return ""
	} else {
		defer file.Close()
		buffer := make([]byte, 256)
		if nRead, err := file.Read(buffer); err != nil {
			return ""
		} else if nRead < 1 {
			return ""
		} else {
			return ExtractSerialNumber(
				strings.TrimSpace(string(buffer[:nRead])))
		}
	}
}

func readSystemSerial() string {
	if serial := readSerialFile(productSerialFile); serial != "" {
		return serial
	}
	return readSerialFile(boardSerialFile)
}
