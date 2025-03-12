package pathregexp

import (
	"errors"
	"regexp"
	"strings"
)

type containsMatcher string

type exactMatcher string

type prefixMatcher string

type prefixRegexpMatcher struct {
	prefix string
	regexp *regexp.Regexp
}

type treeMatcher string

func compile(expression string) (Regexp, error) {
	if expression == "" {
		return nil, errors.New("empty expression")
	}
	if matcher := compileContainsMatcher(expression); matcher != "" {
		return matcher, nil
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
		if ch == '.' &&
			builder.Len() > 1 &&
			index+2 < length &&
			expression[index+1] == '*' {
			re, err := regexp.Compile("^" + expression[index:])
			if err != nil {
				return nil, err
			}
			return &prefixRegexpMatcher{
				prefix: builder.String(),
				regexp: re,
			}, nil
		}
		if expression[index:] == "(|/.*)$" && builder.Len() > 0 {
			return treeMatcher(builder.String()), nil
		}
		return regexp.Compile("^" + expression)
	}
	return prefixMatcher(builder.String()), nil
}

func compileContainsMatcher(expression string) containsMatcher {
	if len(expression) < 6 {
		return ""
	}
	if !strings.HasPrefix(expression, "/.*") {
		return ""
	}
	if !strings.HasSuffix(expression, ".*") {
		return ""
	}
	builder := &strings.Builder{}
	length := len(expression) - 2
	for index := 3; index < length; index++ {
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
		return ""
	}
	if builder.Len() > 0 {
		return containsMatcher(builder.String())
	}
	return ""
}

func isOptimised(regex Regexp) bool {
	_, isRegex := regex.(*regexp.Regexp)
	return !isRegex
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

func (m containsMatcher) MatchString(s string) bool {
	if len(s) < 1 {
		return false
	}
	if s[0] != '/' {
		return false
	}
	return strings.Contains(s, string(m))
}

func (m exactMatcher) MatchString(s string) bool {
	return string(m) == s
}

func (m prefixMatcher) MatchString(s string) bool {
	return strings.HasPrefix(s, string(m))
}

func (m *prefixRegexpMatcher) MatchString(s string) bool {
	if !strings.HasPrefix(s, m.prefix) {
		return false
	}
	return m.regexp.MatchString(s[len(m.prefix):])
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
