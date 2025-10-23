package manager

import (
	"bufio"
	"fmt"
	"os"
	"unicode"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

type bufferedFile struct {
	*os.File
	*bufio.Reader
}

// multiRename will perform multiple renames. If restoreOnFailure is true it
// will attempt to revert (undo) if there are failures.
// The keys in renameMap are the old paths and the values are the new paths.
func multiRename(renameMap map[string]string, restoreOnFailure bool) error {
	restoreMap := make(map[string]string, len(renameMap))
	for oldPath, newPath := range renameMap {
		if err := os.Rename(oldPath, newPath); err != nil {
			if restoreOnFailure {
				if err := multiRename(restoreMap, false); err != nil {
					return err
				}
			}
			return err
		}
		restoreMap[newPath] = oldPath
	}
	return nil
}

func openBufferedFile(filename string) (*bufferedFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return &bufferedFile{
		File:   file,
		Reader: bufio.NewReader(file),
	}, nil
}

func restore(oldName, newName string, retain bool) error {
	if retain {
		return fsutil.CopyFile(newName, oldName, fsutil.PrivateFilePerms)
	} else {
		return os.Rename(oldName, newName)
	}

}

func (r *bufferedFile) Read(p []byte) (int, error) {
	return r.Reader.Read(p)
}

func validateHostname(hostname string) error {
	for _, char := range hostname {
		if unicode.IsLetter(char) ||
			unicode.IsNumber(char) {
			continue
		}
		switch char {
		case '-', '.':
			continue
		}
		return fmt.Errorf("hostname: %s contains invalid character: %c",
			hostname, char)
	}
	return nil
}
