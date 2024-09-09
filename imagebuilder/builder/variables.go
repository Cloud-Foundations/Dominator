package builder

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/json"
)

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
