package flagutil

import (
	"strings"
)

func (ss *StringSet) String() string {
	retval := &strings.Builder{}
	for str := range *ss {
		if retval.Len() > 0 {
			retval.WriteRune(',')
		}
		retval.WriteString(str)
	}
	return retval.String()
}

func (ss *StringSet) Set(value string) error {
	*ss = make(StringSet)
	for _, str := range strings.Split(value, ",") {
		(*ss)[str] = struct{}{}
	}
	return nil
}
