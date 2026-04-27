package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSymlinkWithInRoot resolves the symlink at path, following the entire
// chain and guarantees the resolved path stays within root. If the path is not
// a symlink ( or does not exist ), it is returned unchanged.
// A dangling symlink or a symlink chain whose final target escapes root
// returns an error.
func resolveSymlinkWithInRoot(root, path string) (string, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", err
	}
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	// Resolve the entire symlink chain. EvalSymlinks returns an error
	// for danling symlinks and symlink loops.
	resolvedTargetPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolvedTargetPath)
	if err != nil {
		return "", fmt.Errorf(
			"compute relative path of %q in %q: %w",
			resolvedTargetPath, root, err,
		)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		targetPath, _ := os.Readlink(path)
		return "", fmt.Errorf(
			"symlink %q target %q escapes root %q (resolved=%q)",
			path, targetPath, root, resolvedTargetPath,
		)
	}
	return resolvedTargetPath, nil
}
