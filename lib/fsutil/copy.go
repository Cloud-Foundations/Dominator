package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func copyToFile(destFilename string, perm os.FileMode, reader io.Reader,
	length uint64) error {
	tmpFilename := destFilename + "~"
	destFile, err := os.OpenFile(tmpFilename,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFilename)
	defer destFile.Close()
	if err := copyToWriter(destFile, tmpFilename, reader, length); err != nil {
		return err
	}
	if err := destFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpFilename, destFilename)
}

func copyToFileExclusive(destFilename string, perm os.FileMode,
	reader io.Reader, length uint64) error {
	// First do a read-only test for existence, to limit file-system mutations.
	if _, err := os.Stat(destFilename); err == nil {
		return os.ErrExist
	}
	tmpFilename := destFilename + "~"
	destFile, err := os.OpenFile(tmpFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		perm)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFilename)
	defer destFile.Close()
	// At this point we own the tmpfile and implicitly the destfile. Do a quick
	// check so that we don't waste time writing if it's going to fail later.
	if _, err := os.Stat(destFilename); err == nil {
		return os.ErrExist
	}
	if err := copyToWriter(destFile, tmpFilename, reader, length); err != nil {
		return err
	}
	if err := destFile.Close(); err != nil {
		return err
	}
	return os.Link(tmpFilename, destFilename)
}

func copyToWriter(writer io.Writer, filename string, reader io.Reader,
	length uint64) error {
	if length < 1 {
		if _, err := io.Copy(writer, reader); err != nil {
			return fmt.Errorf("error copying: %s", err)
		}
	} else {
		length := int64(length)
		if nCopied, err := io.CopyN(writer, reader, length); err != nil {
			return fmt.Errorf("error copying: %s", err)
		} else if nCopied != length {
			return fmt.Errorf("expected length: %d, got: %d for: %s\n",
				length, nCopied, filename)
		}
	}
	return nil
}

func copyTree(destDir, sourceDir string, allTypes bool,
	copyFunc func(destFilename, sourceFilename string,
		mode os.FileMode) error) error {
	file, err := os.Open(sourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return err
	}
	for _, name := range names {
		sourceFilename := path.Join(sourceDir, name)
		destFilename := path.Join(destDir, name)
		var stat wsyscall.Stat_t
		if err := wsyscall.Lstat(sourceFilename, &stat); err != nil {
			return errors.New(sourceFilename + ": " + err.Error())
		}
		switch stat.Mode & wsyscall.S_IFMT {
		case wsyscall.S_IFDIR:
			if err := os.Mkdir(destFilename, DirPerms); err != nil {
				if !os.IsExist(err) {
					return err
				}
			}
			err := copyTree(destFilename, sourceFilename, allTypes, copyFunc)
			if err != nil {
				return err
			}
		case wsyscall.S_IFREG:
			err := copyFunc(destFilename, sourceFilename,
				os.FileMode(stat.Mode)&os.ModePerm)
			if err != nil {
				return err
			}
		case wsyscall.S_IFLNK:
			if !allTypes {
				continue
			}
			sourceTarget, err := os.Readlink(sourceFilename)
			if err != nil {
				return errors.New(sourceFilename + ": " + err.Error())
			}
			if destTarget, err := os.Readlink(destFilename); err == nil {
				if sourceTarget == destTarget {
					continue
				}
			}
			os.Remove(destFilename)
			if err := os.Symlink(sourceTarget, destFilename); err != nil {
				return err
			}
		default:
			return errors.New("unsupported file type")
		}
	}
	return nil
}

func copyFile(destFilename, sourceFilename string, mode os.FileMode,
	exclusive bool) error {
	if mode == 0 {
		var stat wsyscall.Stat_t
		if err := wsyscall.Stat(sourceFilename, &stat); err != nil {
			return errors.New(sourceFilename + ": " + err.Error())
		}
		mode = os.FileMode(stat.Mode & wsyscall.S_IFMT)
	}
	sourceFile, err := os.Open(sourceFilename)
	if err != nil {
		return errors.New(sourceFilename + ": " + err.Error())
	}
	defer sourceFile.Close()
	if exclusive {
		return CopyToFileExclusive(destFilename, mode, sourceFile, 0)
	}
	return CopyToFile(destFilename, mode, sourceFile, 0)
}
