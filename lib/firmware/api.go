package firmware

// ExtractSerialNumber will extract a valid product serial number from a raw
// serial number. If the input does not contain a valid serial number, the empty
// string is returned.
func ExtractSerialNumber(input string) string {
	return extractSerialNumber(input)
}

// ReadSystemSerial will read the product serial number and if not valid/found
// will fall back to reading the board serial number. If there is no valid
// serial number found, the empty string is returned.
func ReadSystemSerial() string {
	return readSystemSerial()
}
