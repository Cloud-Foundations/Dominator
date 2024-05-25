package flagutil

import (
	"strings"
)

func (ss *StringSet) String() string {
	buffer := &strings.Builder{}
	for str := range *ss {
		if buffer.Len() > 0 {
			buffer.WriteRune(',')
		}
		buffer.WriteString(str)
	}
	return buffer.String()
}

func (ss *StringSet) Set(value string) error {
	*ss = make(StringSet)
	for _, str := range strings.Split(value, ",") {
		(*ss)[str] = struct{}{}
	}
	return nil
}
