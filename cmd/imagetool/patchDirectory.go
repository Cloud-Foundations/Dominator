package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/osutil"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

type flusher interface {
	Flush() error
}

func patchDirectorySubcommand(args []string, logger log.DebugLogger) error {
	if err := patchDirectory(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error patching directory: %s", err)
	}
	return nil
}

func patchDirectory(imageName, dirName string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	var triggersRunner sublib.TriggersRunner
	if *runTriggers {
		if dirName != "/" {
			return errors.New("directory must be / when running triggers")
		}
		goRoutine := goroutine.New()
		triggersRunner = func(triggers []*triggers.Trigger, action string,
			logger log.Logger) bool {
			var retval bool
			goRoutine.Run(func() {
				retval = runTriggersFunc(triggers, action, logger)
			})
			return retval
		}
	}
	logger.Debugf(0, "getting image: %s\n", imageName)
	img, imageName, err := getTypedImageAndName(imageName)
	if err != nil {
		return err
	}
	if *ignoreFilters {
		img.Filter = nil
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
			rootDir, triggersRunner, logger)
	}()
	return <-errorChannel
}

func patchRoot(img *image.Image, objectsGetter objectserver.ObjectsGetter,
	imageName, dirName, rootDir string, triggersRunner sublib.TriggersRunner,
	logger log.DebugLogger) error {
	if err := wsyscall.UnshareMountNamespace(); err != nil {
		return fmt.Errorf("unable to unshare mount namesace: %s", err)
	}
	wsyscall.Unmount(rootDir, 0)
	err := wsyscall.Mount(dirName, rootDir, "", wsyscall.MS_BIND, "")
	if err != nil {
		return fmt.Errorf("unable to bind mount %s to %s: %s",
			dirName, rootDir, err)
	}
	logger.Debugf(0, "scanning directory: %s\n", dirName)
	startTime := time.Now()
	sfs, err := scanner.ScanFileSystem(rootDir, nil, img.Filter, nil, nil, nil)
	if err != nil {
		return err
	}
	logger.Debugf(0, "scanned in %s\n", format.Duration(time.Since(startTime)))
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
	startTime = time.Now()
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
	logger.Debugln(0, "starting update")
	startTime = time.Now()
	_, _, err = sublib.UpdateWithOptions(subRequest, sublib.UpdateOptions{
		Logger:            logger,
		ObjectsDir:        objectsDir,
		RootDirectoryName: rootDir,
		RunTriggers:       triggersRunner,
	})
	if err != nil {
		return err
	}
	logger.Debugf(0, "updated in %s\n", format.Duration(time.Since(startTime)))
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

// Returns true if there were failures.
func runTriggersFunc(triggerList []*triggers.Trigger, action string,
	logger log.Logger) bool {
	// First process reboot triggers. Process them all.
	var doReboot, hadFailures bool
	for _, trigger := range triggerList {
		if trigger.DoReboot {
			doReboot = true
			if !osutil.RunCommand(logger, "service", trigger.Service,
				"restart") {
				hadFailures = true
			}
		}
	}
	if hadFailures {
		logger.Println("Failures preparing for reboot. Fix and then reboot")
		return true
	}
	if doReboot {
		logger.Println("Rebooting")
		failureChannel := osutil.RunCommandBackground(logger, "reboot", "-f")
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-failureChannel:
			logger.Println("Reboot failed, trying harder")
		case <-timer.C:
			logger.Println("Still alive after 30 seconds, rebooting harder")
		}
		if logger, ok := logger.(flusher); ok {
			logger.Flush()
		}
		time.Sleep(time.Second)
		if err := osutil.HardReboot(logger); err != nil {
			logger.Printf("Hard reboot failed: %s\n", err)
		}
		return true
	}
	for _, trigger := range triggerList {
		logger.Printf("Action: service %s %s\n", trigger.Service, "restart")
		if !osutil.RunCommand(logger, "service", trigger.Service, "restart") {
			hadFailures = true
		}
	}
	return hadFailures
}
