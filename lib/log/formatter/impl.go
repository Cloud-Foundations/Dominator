package formatter

import (
	"fmt"
	"strings"
)

func formatPairs(pairs []string) string {
	var args []interface{}
	var formatString strings.Builder
	for index := 0; index < len(pairs); index++ {
		if index+1 < len(pairs) {
			format := pairs[index]
			index++
			arg := pairs[index]
			if arg != "" {
				formatString.WriteString(format)
				args = append(args, arg)
			}
		} else {
			formatString.WriteString(pairs[index])
		}
	}
	return fmt.Sprintf(formatString.String(), args...)
}
