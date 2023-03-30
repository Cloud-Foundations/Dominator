package builder

import (
	"testing"
)

var (
	testBuilder     = &Builder{}
	testMappingFunc = func(name string) string {
		return testStream.getenv()[name]
	}
	testStream = &imageStreamType{
		name: "users/fred/generic/base/Debian-10/amd64",
	}
)

func TestSimpleExpressionExpansion(t *testing.T) {
	result := expandExpression("${IMAGE_STREAM}", testMappingFunc)
	if result != testStream.name {
		t.Errorf("expected: %s got: %s", testStream.name, result)
	}
	result = expandExpression("${IMAGE_STREAM_DIRECTORY_NAME}", testMappingFunc)
	expected := "users/fred/generic/base/Debian-10"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
	result = expandExpression("${IMAGE_STREAM_LEAF_NAME}", testMappingFunc)
	expected = "amd64"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestSubExpressionExpansion(t *testing.T) {
	result := expandExpression("${IMAGE_STREAM[/:]}", testMappingFunc)
	if result != testStream.name {
		t.Errorf("expected: %s got: %s", testStream.name, result)
	}
	result = expandExpression("${IMAGE_STREAM[/2:-1]}", testMappingFunc)
	expected := "generic/base/Debian-10"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}
