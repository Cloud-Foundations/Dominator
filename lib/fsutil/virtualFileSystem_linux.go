//go:build linux

package fsutil

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func openRoot(path string) (int, error) {
	return unix.Open(path, unix.O_DIRECTORY|unix.O_PATH|unix.O_CLOEXEC, 0)
}

func secureMkdir(rootFd int, relPath string, mode uint32) error {
	dir, file := filepath.Split(relPath)
	parentFd, err := unix.Openat2(rootFd, dir, &unix.OpenHow{
		Flags:   unix.O_DIRECTORY | unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return fmt.Errorf("resolving parent directory %q: %w", dir, err)
	}
	err = unix.Mkdirat(parentFd, file, mode)
	if err != nil && err != unix.EEXIST {
		return fmt.Errorf("mkdir %q:%w", relPath, err)
	}
	return nil
}

func secureOpenFile(rootFd int, relPath string, mode uint32) (*os.File, error) {
	fileFd, err := unix.Openat2(rootFd, relPath, &unix.OpenHow{
		Flags:   unix.O_RDWR | unix.O_APPEND | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err == nil {
		// file already exists safely.
		return os.NewFile(uintptr(fileFd), relPath), nil
	}
	if err != unix.ENOENT {
		return nil, fmt.Errorf("error resolving file %q securely: %w",
			relPath, err)
	}
	// ENOENT encountered, could be missing file or dangling symlink,
	// check if parent directory exists.
	parentFd, parentErr := unix.Openat2(
		rootFd,
		filepath.Dir(relPath),
		&unix.OpenHow{
			Flags:   unix.O_DIRECTORY | unix.O_PATH | unix.O_CLOEXEC,
			Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
		},
	)
	if parentErr != nil {
		return nil, fmt.Errorf("dangling symlink detected in path: %q", relPath)
	}
	if err := unix.Close(parentFd); err != nil {
		return nil, err
	}
	fileFd, err = unix.Openat2(rootFd, relPath, &unix.OpenHow{
		Flags:   unix.O_RDWR | unix.O_CREAT | unix.O_APPEND | unix.O_CLOEXEC,
		Mode:    uint64(mode),
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("creating/appending file %q: %w", relPath, err)
	}
	return os.NewFile(uintptr(fileFd), relPath), nil
}
