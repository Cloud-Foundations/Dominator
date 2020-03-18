package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

func patchDirectorySubcommand(args []string, logger log.DebugLogger) error {
	if err := patchDirectory(args[0], args[1], logger); err != nil {
		return fmt.Errorf("Error getting image: %s", err)
	}
	return nil
}

func patchDirectory(imageName, dirName string, logger log.DebugLogger) error {
	imageSClient, objectClient := getClients()
	logger.Debugf(0, "getting image: %s\n", imageName)
	img, err := imgclient.GetImage(imageSClient, imageName)
	if err != nil {
		return err
	}
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return err
	}
	img.FileSystem.BuildEntryMap()
	rootDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.Remove(rootDir)
	errorChannel := make(chan error)
	go func() {
		errorChannel <- patchRoot(img, objectClient, imageName, dirName,
			rootDir, logger)
	}()
	return <-errorChannel
}

func patchRoot(img *image.Image, objectsGetter objectserver.ObjectsGetter,
	imageName, dirName, rootDir string, logger log.DebugLogger) error {
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
	sfs, err := scanner.ScanFileSystem(rootDir, nil, img.Filter, nil, nil, nil)
	if err != nil {
		return err
	}
	fs := &sfs.FileSystem
	if err := fs.RebuildInodePointers(); err != nil {
		return err
	}
	fs.BuildEntryMap()
	subObj := domlib.Sub{FileSystem: fs}
	fetchMap, _ := domlib.BuildMissingLists(subObj, img, false, true,
		logger)
	objectsToFetch := objectcache.ObjectMapToCache(fetchMap)
	subdDir := filepath.Join(rootDir, ".subd")
	objectsDir := filepath.Join(subdDir, "objects")
	defer os.RemoveAll(subdDir)
	startTime := time.Now()
	objectsReader, err := objectsGetter.GetObjects(objectsToFetch)
	if err != nil {
		return err
	}
	defer objectsReader.Close()
	logger.Debugf(0, "fetching %d objects", len(objectsToFetch))
	for _, hashVal := range objectsToFetch {
		length, reader, err := objectsReader.NextObject()
		if err != nil {
			return err
		}
		err = readOne(objectsDir, hashVal, length, reader)
		reader.Close()
		if err != nil {
			return err
		}
	}
	logger.Debugf(0, "fetched %d objects in %s",
		len(objectsToFetch), format.Duration(time.Since(startTime)))
	subObj.ObjectCache = append(subObj.ObjectCache, objectsToFetch...)
	var subRequest subproto.UpdateRequest
	if domlib.BuildUpdateRequest(subObj, img, &subRequest, false, true,
		nulllogger.New()) {
		return errors.New("failed building update: missing computed files")
	}
	subRequest.ImageName = imageName
	subRequest.Triggers = nil
	logger.Debugln(0, "starting update")
	_, _, err = sublib.Update(subRequest, rootDir, objectsDir, nil, nil, nil,
		logger)
	if err != nil {
		return err
	}
	return nil
}

func readOne(objectsDir string, hashVal hash.Hash, length uint64,
	reader io.Reader) error {
	filename := filepath.Join(objectsDir, objectcache.HashToFilename(hashVal))
	dirname := filepath.Dir(filename)
	if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
		return err
	}
	return fsutil.CopyToFile(filename, fsutil.PrivateFilePerms, reader, length)
}
