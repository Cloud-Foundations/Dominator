package filter

import (
	"errors"
	"regexp"
	"strings"
)

type exactMatcher string
type prefixMatcher string
type treeMatcher string

func compileMatcher(expression string) (matcherI, error) {
	if expression == "" {
		return nil, errors.New("empty expression")
	}
	builder := &strings.Builder{}
	length := len(expression)
	for index := 0; index < length; index++ {
		ch := expression[index]
		if isPlain(ch) {
			builder.WriteByte(ch)
			continue
		}
		if ch == '[' && index+2 < length && expression[index+2] == ']' {
			if middleChar := expression[index+1]; middleChar != '^' {
				builder.WriteByte(middleChar)
				index += 2
				continue
			}
		}
		if ch == '$' && index == length-1 {
			return exactMatcher(builder.String()), nil
		}
		if ch == '.' && index == length-2 && expression[index+1] == '*' {
			return prefixMatcher(builder.String()), nil
		}
		if expression[index:] == "(|/.*)$" && builder.Len() > 0 {
			return treeMatcher(builder.String()), nil
		}
		return regexp.Compile("^" + expression)
	}
	return prefixMatcher(builder.String()), nil
}

func isPlain(ch byte) bool {
	if '0' <= ch && ch <= '9' {
		return true
	}
	if 'A' <= ch && ch <= 'Z' {
		return true
	}
	if 'a' <= ch && ch <= 'z' {
		return true
	}
	switch ch {
	case '-':
	case '/':
	case '_':
	default:
		return false
	}
	return true
}

func (m exactMatcher) MatchString(s string) bool {
	return string(m) == s
}

func (m prefixMatcher) MatchString(s string) bool {
	return strings.HasPrefix(s, string(m))
}

func (m treeMatcher) MatchString(s string) bool {
	matcherLength := len(m)
	stringLength := len(s)
	if stringLength < matcherLength {
		return false
	}
	if string(m) == s {
		return true
	}
	if stringLength > matcherLength &&
		stringLength > 0 &&
		s[matcherLength] == '/' &&
		s[:matcherLength] == string(m) {
		return true
	}
	return false
}
