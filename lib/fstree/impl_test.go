package fstree

import (
	"testing"
)

const (
	testLine = "file0/0/16893/0/0/12/db3974a97f2407b7cae1ae637c0030687a11913274d578492558e39c16c017de84eacdc8c62fe34ee4e12b4b1428817f09b6a2760c3f8a664ceae94d2434a593"
)

func TestDecodeTreeEntry(t *testing.T) {
	var entry TreeEntry
	if err := decodeTreeEntry(testLine, &entry); err != nil {
		t.Error(err)
	}
}
