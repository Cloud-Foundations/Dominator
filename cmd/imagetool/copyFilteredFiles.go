package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func copyFilteredFilesSubcommand(args []string, logger log.DebugLogger) error {
	if err := copyFilteredFiles(args[0], args[1], args[2], logger); err != nil {
		return fmt.Errorf("error copying filtered files: %s", err)
	}
	return nil
}

func copyFilteredFiles(imageName, sourceDirectory, destDirectory string,
	logger log.DebugLogger) error {
	scanFilter, err := filter.New(scanExcludeList)
	if err != nil {
		return err
	}
	img, err := getImageMetadata(imageName)
	if err != nil {
		return err
	}
	sourceRoot, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.Remove(sourceRoot)
	errorChannel := make(chan error)
	go func() {
		errorChannel <- copyFilteredRoot(scanFilter, img.Filter,
			sourceDirectory, sourceRoot, destDirectory, logger)
	}()
	return <-errorChannel
}

func copyFilteredRoot(scanFilter, imageFilter *filter.Filter,
	sourceDirectory, sourceRoot, destDirectory string,
	logger log.DebugLogger) error {
	return walkFilteredRoot(scanFilter, imageFilter, sourceDirectory,
		sourceRoot,
		func(path string, fi os.FileInfo) error {
			stat, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("bad FileInfo.Sys() type: %T", fi.Sys())
			}
			destpath := filepath.Join(destDirectory, path)
			srcpath := filepath.Join(sourceDirectory, path)
			if fi.IsDir() {
				if err := os.Mkdir(destpath, fi.Mode()); err != nil {
					if !os.IsExist(err) {
						return err
					}
				}
			} else if fi.Mode() & ^os.ModePerm != 0 {
				return nil
			} else {
				err := fsutil.CopyFile(destpath, srcpath, fi.Mode())
				if err != nil {
					return err
				}
			}
			if stat.Uid != 0 || stat.Gid != 0 {
				err := os.Chown(destpath, int(stat.Uid), int(stat.Gid))
				if err != nil {
					return err
				}
			}
			err := os.Chtimes(destpath, fi.ModTime(), fi.ModTime())
			if err != nil {
				return err
			}
			return nil
		},
		logger)
}
