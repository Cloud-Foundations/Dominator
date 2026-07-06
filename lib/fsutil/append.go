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

func appendFile(destFilename, sourceFilename string) error {
	if _, err := os.Stat(destFilename); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Dest file doesn't exist, so just copy the file.
			var err error
			mode, err := getFilePerms(sourceFilename)
			if err != nil {
				return err
			}
			return copyFile(destFilename, sourceFilename, mode, false)
		}
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
	appendFunc func(dest, src string) error) error {
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
				// If path is a directory, create directory and return.
				// WalkDir will automatically visit the children next.
				if err := os.MkdirAll(destFilename, DirPerms); err != nil {
					return err
				}
			case fileType.IsRegular():
				if err := appendFunc(destFilename, path); err != nil {
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
