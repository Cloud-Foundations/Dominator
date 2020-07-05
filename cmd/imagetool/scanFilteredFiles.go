package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func scanFilteredFilesSubcommand(args []string, logger log.DebugLogger) error {
	if err := scanFilteredFiles(args[0], args[1], logger); err != nil {
		return fmt.Errorf("Error scanning filtered files: %s", err)
	}
	return nil
}

func getImageMetadata(imageName string) (*image.Image, error) {
	imageSClient, _ := getClients()
	logger.Debugf(0, "getting image: %s\n", imageName)
	request := proto.GetImageRequest{
		ImageName:        imageName,
		IgnoreFilesystem: true,
		Timeout:          *timeout,
	}
	var reply proto.GetImageResponse
	err := imageSClient.RequestReply("ImageServer.GetImage", request, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Image == nil {
		return nil, fmt.Errorf("image: %s not found", imageName)
	}
	return reply.Image, nil
}

func scanFilteredFiles(imageName, dirName string,
	logger log.DebugLogger) error {
	scanFilter, err := filter.New(scanExcludeList)
	if err != nil {
		return err
	}
	img, err := getImageMetadata(imageName)
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
	runtime.LockOSThread()
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
