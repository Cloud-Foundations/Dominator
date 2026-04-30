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
	var curr string
	linksWalked := 0
	unprocessed := strings.Split(filepath.ToSlash(rel), "/")
	symlinkDepth := 0
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
			curr = filepath.Dir(curr)
			if curr == "." || curr == sep {
				curr = ""
			}
			continue
		}
		curr = filepath.Join(curr, comp)
		hostCurr := filepath.Join(root, curr)
		info, err := os.Lstat(hostCurr)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if isFromSymlink {
					fullTarget := hostCurr
					if symlinkDepth > 0 {
						remainder := unprocessed[:symlinkDepth]
						fullTarget = filepath.Join(
							hostCurr,
							filepath.Join(remainder...),
						)
					}
					return "",
						fmt.Errorf(
							"dangling symlink: %q resolves to missing target %q",
							path,
							fullTarget,
						)
				}
				if len(unprocessed) > 0 {
					curr = filepath.Join(curr, filepath.Join(unprocessed...))
				}
				break
			}
			return "", fmt.Errorf("lstat %q: %w", hostCurr, err)
		}
		// If it's not a symlink, we've found our final destination.
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		linksWalked++
		if linksWalked > maxLinks {
			return "", errors.New("too many symlinks (loop detected)")
		}
		// Read the symlink target.
		target, err := os.Readlink(hostCurr)
		if err != nil {
			return "", err
		}
		// Rebase the target based on absolute vs relative links.
		if filepath.IsAbs(target) {
			volLen := len(filepath.VolumeName(target))
			curr = "" // Absolute links reset back to virtual root.
			target = filepath.Clean(target[volLen:])
		} else {
			curr = filepath.Dir(curr)
		}
		targetComps := strings.Split(filepath.ToSlash(target), "/")
		unprocessed = append(targetComps, unprocessed...)
		symlinkDepth += len(targetComps)
	}
	return filepath.Join(root, curr), nil
}
