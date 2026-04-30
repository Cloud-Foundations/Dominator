package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveSymlinkWithInRoot resolves the symlink at path, following the entire
// chain and guarantees the resolved path stays within root. ".." segments and
// absolute targets are clamped at root (chroot-style semantics), so resolution
// never touches paths outside root.If the path is not a symlink (or does not
// exist), it is returned unchanged. A dangling symlink chain returns an error.
func resolveSymlinkWithInRoot(root, path string) (string, error) {
	const maxLinks = 255
	sep := string(filepath.Separator)
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("relative path of %q in %q: %w", path, root, err)
	}
	//Caller-supplied path must already be inside root; reject before any I/O.
	if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
		return "", fmt.Errorf("path %q escapes root %q", path, root)
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
		// If it's not a symlink, we've found our final destination.
		if info.Mode()&os.ModeSymlink == 0 {
			return hostCurr, nil
		}
		// Read the symlink target.
		target, err := os.Readlink(hostCurr)
		if err != nil {
			return "", err
		}
		// Rebase the target as if root were "/" and let Clean's "/..->/" rule
		// clamp leading ".." segments at root, before they could otherwise
		// collapse against the host filesystem when joined with root on the
		// next iteration.
		var abs string
		if filepath.IsAbs(target) {
			volLen := len(filepath.VolumeName(target))
			abs = filepath.Clean(target[volLen:])
		} else {
			abs = filepath.Clean(
				sep + filepath.Join(filepath.Dir(curr), target),
			)
		}
		curr = strings.TrimPrefix(abs, sep)
		if curr == "" {
			curr = "."
		}
	}
	return "", errors.New("too many symlinks (loop detected)")
}
