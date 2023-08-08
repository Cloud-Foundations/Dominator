package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

func scanFilteredFilesSubcommand(args []string, logger log.DebugLogger) error {
	if err := scanFilteredFiles(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error scanning filtered files: %s", err)
	}
	return nil
}

func scanFilteredFiles(imageName, dirName string,
	logger log.DebugLogger) error {
	scanFilter, err := filter.New(scanExcludeList)
	if err != nil {
		return err
	}
	img, err := getTypedImageMetadata(imageName)
	if err != nil {
		return err
	}
	rootDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.Remove(rootDir)
	errorChannel := make(chan error)
	go func() {
		errorChannel <- scanFilteredRoot(scanFilter, img.Filter, dirName,
			rootDir, logger)
	}()
	return <-errorChannel
}

func scanFilteredRoot(scanFilter, imageFilter *filter.Filter,
	dirName, rootDir string, logger log.DebugLogger) error {
	return walkFilteredRoot(scanFilter, imageFilter, dirName, rootDir,
		func(path string, fi os.FileInfo) error {
			fmt.Println(path)
			return nil
		},
		logger)
}

func walkFilteredRoot(scanFilter, imageFilter *filter.Filter,
	dirName, rootDir string,
	walkFunc func(path string, fi os.FileInfo) error,
	logger log.DebugLogger) error {
	if err := wsyscall.UnshareMountNamespace(); err != nil {
		return fmt.Errorf("unable to unshare mount namesace: %s", err)
	}
	syscall.Unmount(rootDir, 0)
	err := wsyscall.Mount(dirName, rootDir, "", wsyscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("unable to bind mount %s to %s: %s",
			dirName, rootDir, err)
	}
	logger.Debugf(0, "scanning directory: %s\n", dirName)
	startPos := len(rootDir)
	return filepath.Walk(rootDir,
		func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			path = path[startPos:]
			if scanFilter.Match(path) {
				return nil
			}
			if imageFilter.Match(path) {
				if err := walkFunc(path, fi); err != nil {
					return err
				}
			}
			return nil
		})
}
