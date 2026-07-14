//go:build !linux

package fsutil

import (
	"errors"
	"os"
)

func openRoot(path string) (int, error) {
	return 0, errors.New("openRoot is supported in Linux only")
}

func secureMkdir(rootFd int, relPath string, mode uint32) error {
	return errors.New("secureMkdir is supported in Linux only")
}

func secureOpenFile(rootFd int, relPath string, mode uint32) (*os.File, error) {
	return nil, errors.New("secureOpenFile is supported in Linux only")
}
