package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fstree"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func unpackFsTreeSubcommand(args []string, logger log.DebugLogger) error {
	err := unpackFsTree(args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error unpacking FsTree: %s", err)
	}
	return nil
}

func unpackFsTree(topDir, treeUrl string, logger log.DebugLogger) error {
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
	if err := os.Mkdir(topDir, fsutil.DirPerms); err != nil {
		return err
	}
	var numBytes, numFiles, numDirectories, numSymlinks uint64
	var mutex sync.Mutex
	fn := func(getter fstree.Getter, dirname string,
		entry *fstree.TreeEntry) error {
		pathname := filepath.Join(topDir, dirname, entry.Filename)
		switch entry.Type {
		case fstree.TypeBlob:
			mutex.Lock()
			numFiles++
			mutex.Unlock()
			if err := makeFile(getter, pathname, entry); err != nil {
				return err
			}
		case fstree.TypeTree:
			mutex.Lock()
			numDirectories++
			mutex.Unlock()
			if err := os.Mkdir(pathname, fsutil.DirPerms); err != nil {
				return err
			}
		case fstree.TypeSymlink:
			mutex.Lock()
			numSymlinks++
			mutex.Unlock()
			if err := makeSymlink(getter, pathname, entry); err != nil {
				return err
			}
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

func makeFile(g fstree.Getter, pathname string, entry *fstree.TreeEntry) error {
	reader, _, err := g.GetBlobReader(entry.Hash)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.Create(pathname)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func makeSymlink(g fstree.Getter, pathname string,
	entry *fstree.TreeEntry) error {
	data, err := g.GetBlobData(entry.Hash)
	if err != nil {
		return err
	}
	return os.Symlink(string(data), pathname)
}
