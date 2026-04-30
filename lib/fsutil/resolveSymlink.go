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
// never touches paths outside root. If the path is not a symlink (or does not
// exist), it is returned unchanged. A dangling symlink chain returns an error.
func resolveSymlinkWithInRoot(root, path string) (string, error) {
	const maxSymlinks = 255
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := validatePathWithinRoot(root, path)
	if err != nil {
		return "", err
	}
	var curr string
	linksWalked := 0
	symlinkDepth := 0
	unprocessed := strings.Split(filepath.ToSlash(rel), "/")
	for len(unprocessed) > 0 {
		comp := unprocessed[0]
		unprocessed = unprocessed[1:]
		isFromSymlink := symlinkDepth > 0
		if isFromSymlink {
			symlinkDepth--
		}
		if comp == "" || comp == "." {
			continue
		}
		if comp == ".." {
			curr = clampToRoot(curr)
			continue
		}
		curr = filepath.Join(curr, comp)
		hostCurr := filepath.Join(root, curr)
		info, err := os.Lstat(hostCurr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if isFromSymlink {
					return "", buildDanglingSymlinkError(
						path,
						hostCurr,
						unprocessed,
						symlinkDepth,
					)
				}
				// Append remaining components for a new directory path.
				if len(unprocessed) > 0 {
					curr = filepath.Join(curr, filepath.Join(unprocessed...))
				}
				break
			}
			return "", fmt.Errorf("lstat %q: %w", hostCurr, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		linksWalked++
		if linksWalked > maxSymlinks {
			return "", errors.New("too many symlinks (loop detected)")
		}
		var targetComps []string
		curr, targetComps, err = evaluateSymlinkTarget(hostCurr, curr)
		if err != nil {
			return "", err
		}
		unprocessed = append(targetComps, unprocessed...)
		symlinkDepth += len(targetComps)
	}
	return filepath.Join(root, curr), nil
}

// validatePathWithinRoot ensures the relative path does not escape the root.
func validatePathWithinRoot(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("relative path of %q in %q: %w", path, root, err)
	}
	sep := string(filepath.Separator)
	if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
		return "", fmt.Errorf("path %q escapes root %q", path, root)
	}
	return rel, nil
}

// clampToRoot emulates a chroot boundary for ".." traversals.
func clampToRoot(curr string) string {
	curr = filepath.Dir(curr)
	if curr == "." || curr == string(filepath.Separator) {
		return ""
	}
	return curr
}

// evaluateSymlinkTarget reads the symlink and rebases the current path.
func evaluateSymlinkTarget(hostCurr, curr string) (string, []string, error) {
	target, err := os.Readlink(hostCurr)
	if err != nil {
		return "", nil, err
	}
	if filepath.IsAbs(target) {
		volLen := len(filepath.VolumeName(target))
		curr = ""
		target = filepath.Clean(target[volLen:])
	} else {
		curr = filepath.Dir(curr)
	}
	targetComps := strings.Split(filepath.ToSlash(target), "/")
	return curr, targetComps, nil
}

// buildDanglingSymlinkError reconstructs the full target path for error
// reporting.
func buildDanglingSymlinkError(
	originalPath, hostCurr string,
	unprocessed []string,
	symlinkDepth int,
) error {
	fullTarget := hostCurr
	if symlinkDepth > 0 && len(unprocessed) >= symlinkDepth {
		remainder := unprocessed[:symlinkDepth]
		fullTarget = filepath.Join(hostCurr, filepath.Join(remainder...))
	}
	return fmt.Errorf(
		"dangling symlink: %q resolves to missing target %q",
		originalPath,
		fullTarget,
	)
}
