package format

import (
	"testing"
)

type milliTestcase struct {
	value    uint64
	expected string
}

func TestMilli(t *testing.T) {
	testcases := []milliTestcase{
		{1, "0.001"},
		{2, "0.002"},
		{10, "0.01"},
		{50, "0.05"},
		{99, "0.099"},
		{100, "0.1"},
		{102, "0.102"},
		{123, "0.123"},
		{999, "0.999"},
		{1000, "1"},
		{1001, "1.001"},
		{2345, "2.345"},
		{9000, "9"},
		{9009, "9.009"},
		{9090, "9.09"},
		{9900, "9.9"},
		{10000, "10"},
		{10012, "10.012"},
	}
	for _, testcase := range testcases {
		field, result := formatMilli(testcase.value)
		if result != testcase.expected {
			t.Errorf(
				"input: %d, field: \"%s\", expected: \"%s\" != result: \"%s\"",
				testcase.value, field, testcase.expected, result)
		}
	}
}
