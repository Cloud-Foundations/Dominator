package builder

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/json"
)

// expandExpression will expand the expression specified by expr and will
// perform parameter expansion on each sub-expression. The mappingFunc is used
// to lookup variables.
func expandExpression(expr string, mappingFunc func(string) string) string {
	return os.Expand(expr, func(parameter string) string {
		return expandVariable(parameter, mappingFunc)
	})
}

// expandVariable will expand the contents of the variable. If the specified
// variable is immediately followed by [<sep><start>:<end>] then it is split by
// the sep character, and then the components from start to end are joined.
// For example, [/2:-1] will remove the first two and last pathname components.
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

func (b *Builder) getVariableFunc(
	extraVariables0, extraVariables1 map[string]string) func(string) string {
	return func(varName string) string {
		if extraVariables0 != nil {
			if varValue, ok := extraVariables0[varName]; ok {
				return varValue
			}
		}
		if extraVariables1 != nil {
			if varValue, ok := extraVariables1[varName]; ok {
				return varValue
			}
		}
		return b.getVariables()[varName]
	}
}

func (b *Builder) getVariables() map[string]string {
	b.variablesLock.RLock()
	defer b.variablesLock.RUnlock()
	return b.variables
}

func (b *Builder) readVariables(readCloser io.ReadCloser) error {
	defer readCloser.Close()
	var variables map[string]string
	if err := json.Read(readCloser, &variables); err != nil {
		return err
	}
	b.variablesLock.Lock()
	oldVariables := b.variables
	b.variables = variables
	b.variablesLock.Unlock()
	if oldVariables == nil {
		b.logger.Println("Loaded variables")
	} else {
		b.logger.Println("Loaded new variables")
	}
	return nil
}

func (b *Builder) readVariablesLoop(rcChannel <-chan io.ReadCloser) {
	for readCloser := range rcChannel {
		if err := b.readVariables(readCloser); err != nil {
			b.logger.Printf("Error reading variables: %s\n", err)
		}
	}
}

type variablesGetter map[string]string

func (vg variablesGetter) add(key, value string) {
	if value != "" {
		vg[key] = value
	}
}

func (vg variablesGetter) copy() variablesGetter {
	retval := make(variablesGetter, len(vg))
	for key, value := range vg {
		retval[key] = value
	}
	return retval
}

func (vg variablesGetter) getenv() map[string]string {
	return vg
}

func (vg variablesGetter) merge(vgToMerge variablesGetter) {
	for key, value := range vgToMerge {
		vg.add(key, value)
	}
}
