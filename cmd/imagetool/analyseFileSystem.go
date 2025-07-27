package main

import (
	"crypto/sha512"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type analysisDataType struct {
	FileSystemSize   uint64
	Objects          map[hash.Hash]*objectInfoType
	ScanDuration     time.Duration
	TotalObjectsSize uint64
}

type objectInfoType struct {
	Count uint64
	Size  uint64
}

func analyseFileSystemSubcommand(args []string, logger log.DebugLogger) error {
	if err := analyseFileSystem(args[0], logger); err != nil {
		return fmt.Errorf("error analysing file-system: %s", err)
	}
	return nil
}

func analyseFileSystem(dirName string, logger log.DebugLogger) error {
	rootDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.Remove(rootDir)
	if err := wsyscall.UnshareMountNamespace(); err != nil {
		return fmt.Errorf("unable to unshare mount namesace: %s", err)
	}
	wsyscall.Unmount(rootDir, 0)
	err = wsyscall.Mount(dirName, rootDir, "", wsyscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("unable to bind mount %s to %s: %s",
			dirName, rootDir, err)
	}
	defer wsyscall.Unmount(rootDir, 0)
	logger.Debugf(0, "scanning directory: %s (bind mounted from: %s)\n",
		rootDir, dirName)
	analysisData := analysisDataType{
		Objects: make(map[hash.Hash]*objectInfoType),
	}
	var statbufFS wsyscall.Statfs_t
	if err := wsyscall.Statfs(rootDir, &statbufFS); err != nil {
		return err
	}
	analysisData.FileSystemSize = statbufFS.Bsize * statbufFS.Blocks
	inodesVisited := make(map[uint64]struct{})
	startTime := time.Now()
	err = filepath.Walk(rootDir,
		func(path string, fi os.FileInfo, err error) error {
			if !fi.Mode().IsRegular() {
				return nil
			}
			if fi.Size() < 1 {
				return nil
			}
			var statbuf wsyscall.Stat_t
			if err := wsyscall.ConvertStat(&statbuf, fi.Sys()); err != nil {
				return fmt.Errorf("no stat data for: %s: %s", path, err)
			}
			if _, visited := inodesVisited[statbuf.Ino]; visited {
				return nil
			}
			inodesVisited[statbuf.Ino] = struct{}{}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			hasher := sha512.New()
			_, err = io.Copy(hasher, file)
			if err != nil {
				return err
			}
			var hashVal hash.Hash
			copy(hashVal[:], hasher.Sum(nil))
			if object := analysisData.Objects[hashVal]; object == nil {
				analysisData.Objects[hashVal] = &objectInfoType{
					Count: 1,
					Size:  uint64(fi.Size()),
				}
				analysisData.TotalObjectsSize += uint64(fi.Size())
			} else if object.Size != uint64(fi.Size()) {
				return fmt.Errorf("size mismatch for: %0x", hashVal)
			} else {
				object.Count++
			}
			return nil
		})
	if err != nil {
		return err
	}
	timeTaken := time.Since(startTime)
	analysisData.ScanDuration = timeTaken
	logger.Debugf(0, "scanned in: %s\n", format.Duration(timeTaken))
	return json.WriteWithIndent(os.Stdout, "    ", analysisData)
}
