package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		return
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
	// Setup source file.
	sourceTmp := t.TempDir()
	destTmp := t.TempDir()
	var (
		sourceDir      = filepath.Join(sourceTmp, "source/dir1")
		destDir        = filepath.Join(destTmp, "dest/dir2/dir3")
		filename       = "test.txt"
		sourceFileData = []byte(
			"#/usr/bin/bash\nVAR1=$(which bash)\necho $VAR1\nthis is \n\ttest data\n",
		)
		expectedDestFileData             = sourceFileData
		filePerms            os.FileMode = 0600
	)
	sourceFilePath := filepath.Join(sourceDir, filename)
	destFilePath := filepath.Join(destDir, filename)
	createBaseDirectory(t, sourceFilePath, 0755)
	// Create source file with data.
	if err := copyToFile(sourceFilePath, 0600,
		bytes.NewReader(sourceFileData), 0); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	// Skipping creation of dest file path.
	// Check dest file doesn't exist before append.
	_, err := os.Stat(destFilePath)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destfile exists already\n")
	}
	if err := AppendTree(filepath.Dir(destFilePath),
		filepath.Dir(sourceFilePath)); err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	f, _ := os.OpenFile(destFilePath, os.O_RDONLY, 0)
	d, _ := io.ReadAll(f)
	t.Logf("file content is \n%s\n", string(d))
	// Check file perm of dest, it should be same as source.
	mode, err := getFilePerms(destFilePath)
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
	// Dest file should exist.
	same, err := CompareFile(expectedDestFileData, destFilePath)
	if err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	if !same {
		t.Fatalf("contents mismatch after append")
	}
}

func TestAppendFileWithExistingDestFile(t *testing.T) {
	// Setup source file.
	sourceTmp := t.TempDir()
	destTmp := t.TempDir()
	var (
		sourceDir      = filepath.Join(sourceTmp, "source/dir1")
		destDir        = filepath.Join(destTmp, "dest/dir2/dir3")
		filename       = "test.txt"
		sourceFileData = []byte(
			"#/usr/bin/bash\nVAR1=$(which bash)\necho $VAR1\nthis is \n\ttest data\n",
		)
		destFileData = []byte(
			"#/usr/bin/python\necho 'this is test data'\n",
		)
		expectedDestFileData = append(destFileData, sourceFileData...)
	)
	sourceFilePath := filepath.Join(sourceDir, filename)
	destFilePath := filepath.Join(destDir, filename)
	createBaseDirectory(t, sourceFilePath, 0755)
	createBaseDirectory(t, destFilePath, 0755)
	// Create source file with data.
	if err := copyToFile(
		sourceFilePath,
		PublicFilePerms,
		bytes.NewReader(sourceFileData),
		0,
	); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	// Create dest file with data.
	if err := copyToFile(
		destFilePath,
		PublicFilePerms,
		bytes.NewReader(destFileData),
		0,
	); err != nil {
		t.Fatalf("error creating dest file %s: %s\n", destFilePath, err.Error())
	}
	if err := AppendTree(filepath.Dir(destFilePath),
		filepath.Dir(sourceFilePath)); err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	f, _ := os.OpenFile(destFilePath, os.O_RDONLY, 0)
	d, _ := io.ReadAll(f)
	t.Logf("file content is \n%s\n", string(d))
	// Dest file should exist.
	same, err := CompareFile(expectedDestFileData, destFilePath)
	if err != nil {
		t.Fatalf("error appending to file: %s\n", err.Error())
	}
	if !same {
		t.Fatalf("contents mismatch after append")
	}
}

func TestAppendFileWithDanglingDestSymlinks(t *testing.T) {
	sourceTmp := t.TempDir()
	destTmp := t.TempDir()
	var (
		sourceDir      = filepath.Join(sourceTmp, "etc/config")
		destDir        = filepath.Join(destTmp, "etc/config")
		filename       = "test.txt"
		sourceFilePath = filepath.Join(sourceDir, filename)
		destFilePath   = filepath.Join(destDir, filename)
	)
	danglingSymlinkPaths :=
		[]struct {
			name,
			danglingSymlinkPath,
			// Resolved path in root with clamping.
			resolvedDanglingSymlinkPath string
		}{
			{
				name:                "relative_path_1",
				danglingSymlinkPath: "../run/systemd/test-service.txt",
				resolvedDanglingSymlinkPath: filepath.Join(
					destTmp,
					"etc/run/systemd/test-service.txt",
				),
			},
			{
				name:                "relative_path_2",
				danglingSymlinkPath: "../../run/systemd/test-service.txt",
				resolvedDanglingSymlinkPath: filepath.Join(
					destTmp,
					"run/systemd/test-service.txt",
				),
			},
			{
				name:                "relative_path_clamp_root",
				danglingSymlinkPath: "../../../../../../run/systemd/test-service.txt",
				resolvedDanglingSymlinkPath: filepath.Join(
					destTmp,
					"run/systemd/test-service.txt",
				),
			},
		}
	createBaseDirectory(t, sourceFilePath, 0755)
	createBaseDirectory(t, destFilePath, 0755)
	// Create source file with data.
	if err := copyToFile(
		sourceFilePath,
		PublicFilePerms,
		bytes.NewReader([]byte{}),
		0,
	); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	for _, danglingSymlink := range danglingSymlinkPaths {
		t.Run(danglingSymlink.name, func(t *testing.T) {
			// Setup dangling symlink at destFile.
			if err := os.Symlink(danglingSymlink.danglingSymlinkPath,
				destFilePath); err != nil {
				t.Fatalf("error creating dangling symlink: %s", err)
			}
			defer func() {
				err := os.Remove(destFilePath)
				if err != nil {
					t.Fatalf("error removing symlink: %s", err)
				}
			}()
			err := AppendTree(destTmp, sourceTmp)
			if err == nil {
				t.Fatalf("expected error for dangling symlinks")
			}
			fmt.Println(err.Error())
			if !strings.EqualFold(err.Error(),
				fmt.Sprintf(
					"dangling symlink: %q resolves to missing target %q",
					destFilePath,
					danglingSymlink.resolvedDanglingSymlinkPath,
				),
			) {
				t.Fatalf("unexpected error")
			}
		})
	}
}

func TestAppendFileWithClampingTargetSymlinks(t *testing.T) {
	sourceTmp := t.TempDir()
	destTmp := t.TempDir()
	var (
		sourceDir      = filepath.Join(sourceTmp, "etc/config")
		destDir        = filepath.Join(destTmp, "etc/config")
		filename       = "test.txt"
		sourceFilePath = filepath.Join(sourceDir, filename)
		sourceData     = []byte(
			"#/usr/bin/bash\n\tThis is test data from source\n",
		)
		destData = []byte(
			"#/usr/bin/bash\n\tThis is test data from dest\n",
		)
		destFilePath        = filepath.Join(destDir, filename)
		danglingSymlinkPath = "../../../run/systemd/test-service.txt"
		expectedData        = append(destData, sourceData...)
	)
	createBaseDirectory(t, sourceFilePath, 0755)
	createBaseDirectory(t, destFilePath, 0755)
	symlinkTargetFullPath := filepath.Clean(
		filepath.Join(destDir,
			strings.TrimPrefix(
				danglingSymlinkPath,
				".."+string(filepath.Separator),
			),
		),
	)
	fmt.Println(symlinkTargetFullPath)
	createBaseDirectory(t, symlinkTargetFullPath, 0755)
	// Create source file with data.
	if err := copyToFile(
		sourceFilePath,
		PublicFilePerms,
		bytes.NewReader(sourceData),
		0,
	); err != nil {
		t.Fatalf("error creating source file %s: %s\n",
			sourceFilePath, err.Error())
	}
	if err := copyToFile(
		symlinkTargetFullPath,
		PublicFilePerms,
		bytes.NewReader(destData),
		0,
	); err != nil {
		t.Fatalf("error creating symlink target file %s: %s\n",
			symlinkTargetFullPath, err.Error())
	}
	if err := os.Symlink(danglingSymlinkPath, destFilePath); err != nil {
		t.Fatalf("error creating dangling symlink: %s", err)
	}
	err := AppendTree(destTmp, sourceTmp)
	if err != nil {
		t.Fatalf("unexpected error in AppendTree: %s", err)
	}
	f, err := os.OpenFile(symlinkTargetFullPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("error opening %s: %s", symlinkTargetFullPath, err)
	}
	d, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("error reading data from %s: %s", symlinkTargetFullPath, err)
	}
	t.Logf("file content is \n%s\n", string(d))
	same, err := compareFile(expectedData, symlinkTargetFullPath)
	if err != nil {
		t.Fatalf("error comparing to file %s: %s", symlinkTargetFullPath, err)
	}
	if !same {
		t.Fatalf(
			"mismatched contents, present: %s\nexpected: %s",
			expectedData, d,
		)
	}
	//Check if symlink is intact.
	expectedSymlinkPath, err := os.Readlink(destFilePath)
	if err != nil {
		t.Fatalf("unexpected error with symlink %s: %s", destFilePath, err)
	}
	if expectedSymlinkPath != danglingSymlinkPath {
		t.Fatalf("symlink is broken.")
	}
}

func TestAppendFileWithExistingTargetSymlinks(t *testing.T) {
	var (
		filename     = "test.txt"
		symlinkPaths = map[string]string{
			"relPathTest": "../../run/systemd/new-rel-test-file.txt",
			"absPathTest": "/var/run/systemd/new-abs-test-file.txt",
		}
		sourceData = []byte("This is from source\n")
		destData   = []byte(
			"#/usr/bin/bash\necho 'hello world'\n\tThis is from symlink",
		)
		expectedData = append(destData, sourceData...)
	)
	for name, symlinkPath := range symlinkPaths {
		t.Run(name, func(t *testing.T) {
			sourceTmp := t.TempDir()
			destTmp := t.TempDir()
			var (
				sourceDir      = filepath.Join(sourceTmp, "etc/config")
				destDir        = filepath.Join(destTmp, "etc/config")
				sourceFilePath = filepath.Join(sourceDir, filename)
				destFilePath   = filepath.Join(destDir, filename)
			)
			createBaseDirectory(t, sourceFilePath, 0755)
			createBaseDirectory(t, destFilePath, 0755)
			var rootDir string
			if !filepath.IsAbs(symlinkPath) {
				rootDir = destDir
			} else {
				rootDir = destTmp
			}
			symlinkTargetFullPath := filepath.Clean(
				filepath.Join(rootDir, symlinkPath),
			)
			createBaseDirectory(t, symlinkTargetFullPath, 0755)
			// Create source file with data.
			if err := copyToFile(
				sourceFilePath,
				PublicFilePerms,
				bytes.NewReader(sourceData),
				0,
			); err != nil {
				t.Fatalf("error creating source file %s: %s\n",
					sourceFilePath, err.Error())
			}
			// Create symlink target path.
			if err := copyToFile(
				symlinkTargetFullPath,
				PublicFilePerms,
				bytes.NewReader(destData),
				0,
			); err != nil {
				t.Fatalf("error creating symlink target file %s: %s\n",
					symlinkTargetFullPath, err.Error())
			}
			// Create symlink for destFilePath to targetPath.
			if err := os.Symlink(symlinkPath, destFilePath); err != nil {
				t.Fatalf("error creating dangling symlink: %s", err)
			}
			err := AppendTree(destTmp, sourceTmp)
			if err != nil {
				t.Fatalf("unexpected error in appendTree: %s", err)
			}
			// If symlink target is absolute, we need to append rootDir
			// for validating data.
			var expectedEvaluatedPath string
			if !filepath.IsAbs(symlinkPath) {
				expectedEvaluatedPath = destFilePath
			} else {
				expectedEvaluatedPath = symlinkTargetFullPath
			}
			// Check if contents match.
			f, err := os.OpenFile(expectedEvaluatedPath, os.O_RDONLY, 0)
			if err != nil {
				t.Fatalf("error opening expected file: %s", err)
			}
			d, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("error reading file contents: %s", err)
			}
			t.Log("file contents is", string(d))
			same, err := compareFile(expectedData, expectedEvaluatedPath)
			if err != nil {
				t.Fatalf("error comparing to file: %s\n", err.Error())
			}
			if !same {
				t.Fatalf("contents mismatch after append")
			}
			// Check if symlink stays intact.
			linkPath, err := os.Readlink(destFilePath)
			if err != nil {
				t.Fatalf(
					"error checking target %s of symlink %s: %s",
					destFilePath,
					symlinkPath,
					err,
				)
			}
			if linkPath != symlinkPath {
				t.Fatalf("symlink targets don't match")
			}
		})
	}
}
