package flagutil

import (
	"errors"
	"sort"
	"strings"
)

func (m *StringToRuneMap) String() string {
	keys := make([]string, 0, len(*m))
	for key := range *m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buffer := &strings.Builder{}
	for _, key := range keys {
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(key)
		buffer.WriteRune(':')
		buffer.WriteRune((*m)[key])
	}
	return buffer.String()
}

func (m *StringToRuneMap) Set(value string) error {
	newMap := make(map[string]rune)
	for _, entry := range strings.Split(value, ",") {
		fields := strings.Split(entry, ":")
		if len(fields) != 2 {
			return errors.New("invalid entry: " + entry)
		}
		if len(fields[1]) != 1 {
			return errors.New("invalid filetype: " + fields[1])
		}
		newMap[fields[0]] = rune(fields[1][0])
	}
	*m = newMap
	return nil
}
