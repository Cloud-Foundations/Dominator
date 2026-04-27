package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSymlinkTargetPath will convert target path of a symlink
// into clean absolute path.
// For example symlink
// /etc/resolv.conf -> ../run/systemd/resolve/stubd-resolv.conf
// will return absolute path /run/systemd/resolve/stubd-resolv.conf.
// If the given file path is not a symlink, we will return error
// fsutil.ErrNotASymlink.
func resolveSymlinkTargetPath(path string) (string, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", err
	}
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		return path, ErrNotASymlink
	}
	targetPath, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(targetPath) {
		return targetPath, nil
	}
	return filepath.Clean(
		filepath.Join(filepath.Dir(path), targetPath),
	), nil
}

// ResolveSymlinkWithInRoot resolves the target of the symlink at path,
// guarantees that the resolved path stays within the root.
// If the given file path is not a symlink, we will return error
// fsutil.ErrNotASymlink.
func resolveSymlinkWithInRoot(root, path string) (string, error) {
	resolvedTargetPath, err := resolveSymlinkTargetPath(path)
	if err != nil {
		if errors.Is(err, ErrNotASymlink) {
			return path, nil
		}
		return "", err
	}
	rel, err := filepath.Rel(root, resolvedTargetPath)
	if err != nil {
		return "", fmt.Errorf(
			"compute relative path of %q in %q: %w",
			resolvedTargetPath, root, err,
		)
	}
	targetPath, _ := os.Readlink(path)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(
			"symlink %q target %q escapes root %q (resolved=%q)",
			path, targetPath, root, resolvedTargetPath,
		)
	}
	return resolvedTargetPath, nil
}
