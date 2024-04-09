package uncommenter

import (
	"bytes"
	"io"
	"testing"
)

var (
	badlyCommentedText = []byte(`First line
,# A comment
Second line
// Another comment
Third line
! More commentary
Final statement.
`)

	badlyCommentedTextExpected = []byte(`First line
,# A comment
Second line
Third line
Final statement.
`)

	properlyCommentedText = []byte(`First line
# A comment
Second line
	// Another comment
Third line
     ! More commentary
Final statement.
`)

	uncommentedText = []byte(`First line
Second line
Third line
Final statement.
`)
)

func TestBad(t *testing.T) {
	reader := New(bytes.NewBuffer(badlyCommentedText), CommentTypeAll)
	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(result, badlyCommentedTextExpected) {
		return
	}
	t.Errorf("Expected: %s, got: %s",
		string(badlyCommentedTextExpected), string(result))
}

func TestGood(t *testing.T) {
	reader := New(bytes.NewBuffer(properlyCommentedText), CommentTypeAll)
	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(result, uncommentedText) {
		return
	}
	t.Errorf("Expected: %s, got: %s",
		string(uncommentedText), string(result))
}
