package expand

// Expression will expand the expression specified by expr and will
// perform parameter expansion on each sub-expression. The mappingFunc is used
// to lookup variables.
func Expression(expr string, mappingFunc func(string) string) string {
	return expandExpression(expr, mappingFunc)
}

// Variable will expand the contents of the variable. If the specified
// variable is immediately followed by [<sep><start>:<end>] then it is split by
// the sep character, and then the components from start to end are joined.
// For example, [/2:-1] will remove the first two and last pathname components.
func Variable(variable string, mappingFunc func(string) string) string {
	return expandVariable(variable, mappingFunc)
}
