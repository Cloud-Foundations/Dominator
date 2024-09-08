package expand

import (
	"testing"
)

var (
	variableMap = map[string]string{
		"IMAGE_STREAM": "users/fred/generic/base/Debian-10/amd64",
	}
)

func testMappingFunc(variable string) string {
	return variableMap[variable]
}

func TestSimpleExpressionExpansion(t *testing.T) {
	result := Expression("${IMAGE_STREAM}", testMappingFunc)
	expected := variableMap["IMAGE_STREAM"]
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestSubExpressionExpansion(t *testing.T) {
	result := expandExpression("${IMAGE_STREAM[/:]}", testMappingFunc)
	expected := variableMap["IMAGE_STREAM"]
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
	result = expandExpression("${IMAGE_STREAM[/2:-1]}", testMappingFunc)
	expected = "generic/base/Debian-10"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestSubExpressionExpansionNegativeIndex(t *testing.T) {
	result := expandExpression("${IMAGE_STREAM[/-2:]}", testMappingFunc)
	expected := "Debian-10/amd64"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}
