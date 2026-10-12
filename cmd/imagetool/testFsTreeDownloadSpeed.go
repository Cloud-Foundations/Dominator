package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fstree"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func testFsTreeDownloadSpeedSubcommand(args []string,
	logger log.DebugLogger) error {
	err := testFsTreeDownloadSpeed(args[0], logger)
	if err != nil {
		return fmt.Errorf("error downloading FsTree: %s", err)
	}
	return nil
}

func testFsTreeDownloadSpeed(treeUrl string, logger log.DebugLogger) error {
	baseUrl, _, err := fstree.SplitTreeUrl(treeUrl)
	if err != nil {
		return err
	}
	getter, err := fstree.NewGetter(fstree.GetterParams{
		BaseUrl:     baseUrl,
		IoSemaphore: make(chan struct{}, 128),
		Logger:      logger,
	})
	if err != nil {
		return err
	}
	var numBytes, numFiles, numDirectories, numSymlinks uint64
	var mutex sync.Mutex
	fn := func(getter fstree.Getter, dirname string,
		entry *fstree.TreeEntry) error {
		switch entry.Type {
		case fstree.TypeBlob:
			mutex.Lock()
			numFiles++
			mutex.Unlock()
			if err := discardFile(getter, entry); err != nil {
				return err
			}
		case fstree.TypeTree:
			mutex.Lock()
			numDirectories++
			mutex.Unlock()
		case fstree.TypeSymlink:
			mutex.Lock()
			numSymlinks++
			mutex.Unlock()
		}
		mutex.Lock()
		numBytes += entry.Size
		mutex.Unlock()
		return nil
	}
	walkParams := fstree.WalkParams{
		Function: fn,
		Getter:   getter,
		Logger:   logger,
		TreeUrl:  treeUrl,
	}
	startTime := time.Now()
	if err := fstree.WalkTree(walkParams); err != nil {
		return err
	}
	duration := time.Since(startTime)
	speed := float64(numBytes) / duration.Seconds()
	logger.Printf("NumBytes: %s in: %ds (%s) (%s/s), numFiles: %d, numDirectories: %d, numSymlinks: %d\n",
		format.FormatBytes(numBytes),
		duration/time.Second,
		format.Duration(duration),
		format.FormatBytes(uint64(speed)),
		numFiles, numDirectories, numSymlinks,
	)
	return nil
}

func discardFile(g fstree.Getter, entry *fstree.TreeEntry) error {
	reader, _, err := g.GetBlobReader(entry.Hash)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(ioutil.Discard, reader)
	return err
}
