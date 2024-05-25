package flagutil

import (
	"strconv"
	"strings"
)

func (ul *UintList) String() string {
	buffer := &strings.Builder{}
	for _, value := range *ul {
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(strconv.FormatUint(uint64(value), 10))
	}
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
