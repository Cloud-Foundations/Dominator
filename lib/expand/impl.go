package expand

import (
	"os"
	"strconv"
	"strings"
)

func expandExpression(expr string, mappingFunc func(string) string) string {
	return os.Expand(expr, func(parameter string) string {
		return expandVariable(parameter, mappingFunc)
	})
}

func expandVariable(variable string, mappingFunc func(string) string) string {
	if len(variable) < 5 {
		return mappingFunc(variable) // Not enough for a sub-expression.
	}
	if variable[len(variable)-1] != ']' {
		return mappingFunc(variable) // Simple variable.
	}
	index := strings.IndexByte(variable, '[')
	if index < 1 {
		return ""
	}
	if len(variable) < index+4 {
		return ""
	}
	variableName := variable[:index]
	separator := variable[index+1 : index+2]
	variableValue := mappingFunc(variableName)
	if variableValue == "" {
		return ""
	}
	splitValue := strings.Split(variableValue, separator)
	splitRange := strings.Split(variable[index+2:len(variable)-1], ":")
	if len(splitRange) != 2 {
		return ""
	}
	var start, end int
	var err error
	if len(splitRange[0]) > 0 {
		start, err = strconv.Atoi(splitRange[0])
		if err != nil {
			return ""
		}
		if start < 0 {
			start += len(splitValue)
		}
	}
	if len(splitRange[1]) > 0 {
		end, err = strconv.Atoi(splitRange[1])
		if err != nil {
			return ""
		}
		if end >= len(splitValue) {
			return ""
		}
		if end < 0 {
			end += len(splitValue)
			if end < start {
				return ""
			}
		}
	} else {
		end = len(splitValue)
	}
	return strings.Join(splitValue[start:end], separator)
}
