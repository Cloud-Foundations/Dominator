package flagutil

import (
	"bytes"
	"strconv"
	"strings"
)

func (ul *UintList) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(`"`)
	for index, value := range *ul {
		buffer.WriteString(strconv.FormatUint(uint64(value), 10))
		if index < len(*ul)-1 {
			buffer.WriteString(",")
		}
	}
	buffer.WriteString(`"`)
	return buffer.String()
}

func (ul *UintList) Set(value string) error {
	newList := make(UintList, 0)
	if value != "" {
		uintStrings := strings.Split(value, ",")
		for _, uintString := range uintStrings {
			if value, err := strconv.ParseUint(uintString, 10, 64); err != nil {
				return err
			} else {
				newList = append(newList, uint(value))
			}
		}
	}
	*ul = newList
	return nil
}
