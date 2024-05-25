package flagutil

import (
	"strings"
)

func (sl *SizeList) String() string {
	buffer := &strings.Builder{}
	for _, size := range *sl {
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(size.String())
	}
	return buffer.String()
}

func (sl *SizeList) Set(value string) error {
	newList := make(SizeList, 0)
	if value != "" {
		sizeStrings := strings.Split(value, ",")
		for _, sizeString := range sizeStrings {
			var size Size
			if err := size.Set(sizeString); err != nil {
				return err
			}
			newList = append(newList, size)
		}
	}
	*sl = newList
	return nil
}
