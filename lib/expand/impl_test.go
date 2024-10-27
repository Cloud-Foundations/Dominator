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

func TestMissingVariableExpression(t *testing.T) {
	result := Expression("prefix${MiSsInG}suffix", testMappingFunc)
	expected := "prefixsuffix"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestMissingVariableOpportunistic(t *testing.T) {
	result := Opportunistic("prefix${MiSsInG}suffix", testMappingFunc)
	expected := "prefix${MiSsInG}suffix"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestSimpleExpressionExpansion(t *testing.T) {
	result := Expression("prefix${IMAGE_STREAM}suffix", testMappingFunc)
	expected := "prefix" + variableMap["IMAGE_STREAM"] + "suffix"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestSimpleOpportunisticExpansion(t *testing.T) {
	result := Opportunistic("prefix${IMAGE_STREAM}suffix", testMappingFunc)
	expected := "prefix" + variableMap["IMAGE_STREAM"] + "suffix"
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
