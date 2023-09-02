package fsutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func readFileTree(topdir, prefix string) (map[string][]byte, error) {
	overlayFiles := make(map[string][]byte)
	startPos := len(topdir) + 1
	err := filepath.Walk(topdir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			overlayFiles[filepath.Join(prefix, path[startPos:])] = data
			return nil
		})
	return overlayFiles, err
}
