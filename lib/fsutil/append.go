package fsutil

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
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

func appendFileWithRoot(rootFd int, destRelPath, sourcePath string) error {
	mode, err := getFilePerms(sourcePath)
	if err != nil {
		return err
	}
	destFile, err := secureOpenFile(rootFd, destRelPath, uint32(mode))
	if err != nil {
		return err
	}
	defer destFile.Close()
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return errors.New(sourcePath + ": " + err.Error())
	}
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf(
			"error copying contents from source %q to dest %q: %w",
			sourcePath, destRelPath, err)
	}
	return nil
}

func appendTree(destDir, sourceDir string,
	appendFunc func(rootFd int, destRelPath, sourcePath string) error) error {
	rootFd, err := openRoot(destDir)
	if err != nil {
		return err
	}
	defer unix.Close(rootFd)
	return filepath.WalkDir(sourceDir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == sourceDir {
				return nil
			}
			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			fileType := d.Type()
			switch {
			case fileType.IsDir():
				return secureMkdir(rootFd, relPath, DirPerms)
			case fileType.IsRegular():
				if err := appendFunc(rootFd, relPath, path); err != nil {
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
