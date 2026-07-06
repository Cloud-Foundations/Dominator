package fsutil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func createBaseDirectory(t *testing.T, path string, perms os.FileMode) {
	// Mark this function as Helper.
	t.Helper()
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			t.Fatalf("path exists but is a file: %s", dir)
		}
	}
	if !os.IsNotExist(err) {
		t.Fatal(err.Error())
	}
	err = os.MkdirAll(dir, perms)
	if err != nil {
		t.Fatalf("error creating directory dir %s: %s", dir, err.Error())
	}
}

func TestAppendFileNonExistingDestFile(t *testing.T) {
	const (
		sourceFileName = "dir1/source"
		destFileName   = "dir2/dir3/dest"
	)
	// setup source file.
	tmp := t.TempDir()
	var (
		sourceFileData = []byte(
			"#/usr/bin/bash\nVAR1=$(which bash)\necho $VAR1\nthis is \n\ttest data\n",
		)
		expectedDestFileData             = sourceFileData
		filePerms            os.FileMode = 0600
	)
	sourceFilePath := filepath.Join(tmp, sourceFileName)
	destFilePath := filepath.Join(tmp, destFileName)
	createBaseDirectory(t, sourceFilePath, 0755)
	// create source file with data.
	if err := copyToFile(sourceFilePath, 0600,
		bytes.NewReader(sourceFileData), 0); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	// skipping creation of dest file path.
	// check dest file doesn't exist before append.
	_, err := os.Stat(destFilePath)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destfile exists already\n")
	}
	if err := AppendTree(filepath.Dir(destFilePath),
		filepath.Dir(sourceFilePath)); err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	// Since Destination file is not present, the entire Tree will be
	// created with source file.
	finalDestPath := filepath.Join(tmp, "dir2/dir3/source")
	f, _ := os.OpenFile(finalDestPath, os.O_RDONLY, 0)
	d, _ := io.ReadAll(f)
	t.Logf("file content is \n%s\n", string(d))
	// check file perm of dest, it should be same as source.
	mode, err := getFilePerms(finalDestPath)
	if err != nil {
		t.Fatalf("error getting dest file perms %s\n", err.Error())
	}
	if mode != filePerms {
		t.Fatalf(
			"dest file perms are not as expected, current: %d, expected: %d\n",
			mode,
			filePerms,
		)
	}
	// dest file should exist.
	same, err := CompareFile(expectedDestFileData, finalDestPath)
	if err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	if !same {
		t.Fatalf("contents mismatch after append")
	}
}

func TestAppendFileWithExistingDestFile(t *testing.T) {
	// setup source file.
	const (
		sourceFileName = "dir1/dir2/dir3/source"
		destFileName   = "dir4/dir2/dir3/dest"
	)
	tmp := t.TempDir()
	var (
		sourceFileData = []byte(
			"#/usr/bin/bash\nVAR1=$(which bash)\necho $VAR1\nthis is \n\ttest data\n",
		)
		destFileData         = []byte("#/usr/bin/python\necho 'this is test data'\n")
		expectedDestFileData = append(destFileData, sourceFileData...)
	)
	sourceFilePath := filepath.Join(tmp, sourceFileName)
	destFilePath := filepath.Join(tmp, destFileName)
	createBaseDirectory(t, sourceFilePath, 0755)
	createBaseDirectory(t, destFilePath, 0755)
	// create source file with data.
	if err := copyToFile(
		sourceFilePath,
		PublicFilePerms,
		bytes.NewReader(sourceFileData),
		0,
	); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	// create dest file with data.
	if err := copyToFile(
		destFilePath,
		PublicFilePerms,
		bytes.NewReader(destFileData),
		0,
	); err != nil {
		t.Fatalf("error creating dest file %s: %s\n", destFilePath, err.Error())
	}
	if err := AppendFile(destFilePath, sourceFilePath); err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	f, _ := os.OpenFile(destFilePath, os.O_RDONLY, 0)
	d, _ := io.ReadAll(f)
	t.Logf("file content is \n%s\n", string(d))
	// dest file should exist.
	same, err := CompareFile(expectedDestFileData, destFilePath)
	if err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	if !same {
		t.Fatalf("contents mismatch after append")
	}
}
