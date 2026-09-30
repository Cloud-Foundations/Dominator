package triggers

import (
	"testing"
)

var (
	testTriggersJson = []byte(`
[
    {
        "MatchLines": [
            "/etc/fileA0",
            "/etc/fileA1"
        ],
        "Service": "svcA"
    },
    {
        "MatchLines": [
            "/usr/fileB0",
            "/usr/fileB1"
        ],
        "Service": "svcB"
    }
]
`)
)

func TestPlainMatch(t *testing.T) {
	trg, err := Decode(testTriggersJson)
	if err != nil {
		t.Fatal(err)
	}
	if err := trg.Compile(); err != nil {
		t.Fatal(err)
	}
	trg.Match("/etc/fileA1")
	trg.Match("/tmp/fileC2")
	matched := trg.GetMatchedTriggers()
	if len(matched) != 1 {
		t.Fatalf("num matched triggers: %d, expected 1", len(matched))
	}
	if matched[0].Service != "svcA" {
		t.Fatalf("matched trigger: %s, expected svcA", matched[0].Service)
	}
	if trg.matchedTriggers != nil {
		t.Fatalf("trg.matchedTriggers: %p, expected nil", trg.matchedTriggers)
	}
	if trg.unmatchedTriggers != nil {
		t.Fatalf("trg.unmatchedTriggers: %p, expected nil",
			trg.unmatchedTriggers)
	}
}

func TestClone(t *testing.T) {
	trg, err := Decode(testTriggersJson)
	if err != nil {
		t.Fatal(err)
	}
	if err := trg.Compile(); err != nil {
		t.Fatal(err)
	}
	trg0 := trg.Clone()
	trg1 := trg.Clone()
	trg0.Match("/etc/fileA1")
	trg0.Match("/tmp/fileC2")
	if trg.matchedTriggers != nil {
		t.Fatalf("trg.matchedTriggers: %p, expected nil", trg.matchedTriggers)
	}
	if trg.unmatchedTriggers != nil {
		t.Fatalf("trg.unmatchedTriggers: %p, expected nil",
			trg.unmatchedTriggers)
	}
	if trg1.matchedTriggers != nil {
		t.Fatalf("trg1.matchedTriggers: %p, expected nil", trg1.matchedTriggers)
	}
	if trg1.unmatchedTriggers != nil {
		t.Fatalf("trg1.unmatchedTriggers: %p, expected nil",
			trg1.unmatchedTriggers)
	}
}
