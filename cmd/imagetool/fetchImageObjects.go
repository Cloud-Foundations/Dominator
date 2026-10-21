package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

func fetchImageObjectsSubcommand(args []string, logger log.DebugLogger) error {
	if err := fetchImageObjects(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error fetching image objects: %s", err)
	}
	return nil
}

func fetchImageObjects(imageName, dirname string, logger log.Logger) error {
	imageSClient, objClient := getClients()
	isDir, err := imgclient.CheckDirectory(imageSClient, imageName)
	if err != nil {
		return err
	}
	if isDir {
		name, err := imgclient.FindLatestImage(imageSClient, imageName, false)
		if err != nil {
			return err
		} else {
			imageName = name
		}
	}
	startTime := time.Now()
	img, err := getTypedImage(imageName)
	if err != nil {
		return err
	}
	finishedTime := time.Now()
	logger.Printf("downloaded image metadata in %s\n",
		format.Duration(finishedTime.Sub(startTime)))
	hashMap := make(map[hash.Hash]struct{}, len(img.FileSystem.InodeTable))
	hashes := make([]hash.Hash, 0, len(img.FileSystem.InodeTable))
	for _, inode := range img.FileSystem.InodeTable {
		if inode, ok := inode.(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				if _, found := hashMap[inode.Hash]; !found {
					hashMap[inode.Hash] = struct{}{}
					hashes = append(hashes, inode.Hash)
				}
			}
		}
	}
	startTime = time.Now()
	objectsReader, err := objClient.GetObjects(hashes)
	if err != nil {
		return err
	}
	defer objectsReader.Close()
	var totalBytes uint64
	for _, hashVal := range hashes {
		filename := filepath.Join(dirname, objectcache.HashToFilename(hashVal))
		if length, err := copyOneObject(filename, objectsReader); err != nil {
			return err
		} else {
			totalBytes += length
		}
	}
	finishedTime = time.Now()
	downloadTime := finishedTime.Sub(startTime)
	downloadSpeed := float64(totalBytes) / downloadTime.Seconds()
	logger.Printf("downloaded %s (%d objects) in %s (%s/s)\n",
		format.FormatBytes(totalBytes), len(hashes),
		format.Duration(downloadTime),
		format.FormatBytes(uint64(downloadSpeed)))
	return nil
}

func copyOneObject(filename string,
	objectsReader objectserver.ObjectsReader) (uint64, error) {
	_, reader, err := objectsReader.NextObject()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	file, err := os.Create(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, err
		}
		dirname := filepath.Dir(filename)
		if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
			return 0, err
		}
		file, err = os.Create(filename)
		if err != nil {
			return 0, err
		}
	}
	defer file.Close()
	nCopied, err := io.Copy(file, reader)
	return uint64(nCopied), err
}
