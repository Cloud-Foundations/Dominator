package fsutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func appendToFile(destFilename string, reader io.Reader,
	length uint64) (err error) {
	destFile, err := os.OpenFile(destFilename, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer func() {
		closeError := destFile.Close()
		// If our function succeeded, but the close failed,
		// return the close error instead.
		if err == nil && closeError != nil {
			err = closeError
		}
	}()
	if err := copyToWriter(destFile, destFilename, reader, length); err != nil {
		return err
	}
	return nil
}

func appendFile(destDir, destFilename, sourceFilename string) error {
	// Resolve the destination path to ensure it stays within destDir.
	// This also safely computes the final path for new files and prevents
	// intermediate directory symlinks for escaping the root boundary.
	destFilename, err := resolveSymlinkWithInRoot(destDir, destFilename)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(destFilename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Dest file doesn't exist, so just copy the file.
			var err error
			mode, err := getFilePerms(sourceFilename)
			if err != nil {
				return err
			}
			return copyFile(destFilename, sourceFilename, mode, false)
		}
		return err
	}
	sourceFile, err := os.Open(sourceFilename)
	if err != nil {
		return errors.New(sourceFilename + ": " + err.Error())
	}
	defer sourceFile.Close()
	// Dest file exists, so append to it.
	return appendToFile(destFilename, sourceFile, 0)
}

func appendTree(destDir, sourceDir string,
	appendFunc func(destDir, dest, src string) error) error {
	return filepath.WalkDir(sourceDir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			destFilename := filepath.Join(destDir, relPath)
			fileType := d.Type()
			switch {
			case fileType.IsDir():
				// Resolve the path to ensure any pre-existing symlinks in the
				// destination are safely clamped to the chroot boundary.
				safeDir, err := resolveSymlinkWithInRoot(destDir,
					destFilename)
				if err != nil {
					return err
				}
				// If path is a directory, create directory and return.
				// WalkDir will automatically visit the children next.
				if err := os.MkdirAll(safeDir, DirPerms); err != nil {
					return err
				}
			case fileType.IsRegular():
				if err := appendFunc(destDir, destFilename, path); err != nil {
					return err
				}
			case fileType&fs.ModeSymlink != 0:
				return errors.New("symlinks are not supported")
			default:
				return fmt.Errorf("unsupported file type: %s",
					fileType.String())
			}
			return nil
		})
}
