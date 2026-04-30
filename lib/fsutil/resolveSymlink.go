package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveSymlinkWithInRoot resolves the symlink at path, following the entire
// chain and guarantees the resolved path stays within root. If the path is not
// a symlink (or does not exist), it is returned unchanged.
// A dangling symlink or a symlink chain whose final target escapes root
// returns an error.
func resolveSymlinkWithInRoot(root, path string) (string, error) {
	const maxLinks = 255
	sep := string(filepath.Separator)
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("relative path of %q in %q: %w", path, root, err)
	}
	curr := rel
	for nlinks := 0; nlinks <= maxLinks; nlinks++ {
		hostCurr := filepath.Join(root, curr)
		info, err := os.Lstat(hostCurr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if nlinks == 0 {
					return hostCurr, nil
				}
				return "",
					fmt.Errorf(
						"dangling symlink: %q resolves to missing target %q",
						path,
						hostCurr,
					)
			}
			return "", fmt.Errorf("lstat %q: %w", hostCurr, err)
		}
		// We only enforce the escape boundary,
		// IF the file physically exists on the disk.
		if curr == ".." || strings.HasPrefix(curr, ".."+sep) {
			return "", fmt.Errorf(
				"path %q evaluates to %q which escapes root %q",
				path, curr, root,
			)
		}
		// If it's not a symlink, we've found our final destination.
		if info.Mode()&os.ModeSymlink == 0 {
			return hostCurr, nil
		}
		// Read the symlink target.
		target, err := os.Readlink(hostCurr)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			volLen := len(filepath.VolumeName(target))
			curr = filepath.Clean(strings.TrimPrefix(target[volLen:], sep))
		} else {
			curr = filepath.Clean(filepath.Join(filepath.Dir(curr), target))
		}
	}
	return "", errors.New("too many symlinks (loop detected)")
}
