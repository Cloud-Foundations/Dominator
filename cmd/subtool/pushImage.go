package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/dom/lib"
	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/client"
)

type nullObjectGetterType struct{}

type timedImageFetch struct {
	image    *image.Image
	duration time.Duration
}

func (getter nullObjectGetterType) GetObject(hashVal hash.Hash) (
	uint64, io.ReadCloser, error) {
	return 0, nil, errors.New("no computed files")
}

func pushImageSubcommand(args []string, logger log.DebugLogger) error {
	startTime := showStart("getSubClient()")
	srpcClient := getSubClientRetry(logger)
	defer srpcClient.Close()
	showTimeTaken(startTime)
	if err := pushImage(srpcClient, args[0]); err != nil {
		return fmt.Errorf("error pushing image: %s: %s", args[0], err)
	}
	return nil
}

func pushImage(srpcClient *srpc.Client, imageName string) error {
	computedInodes := make(map[string]*filesystem.RegularInode)
	// Start querying the imageserver for the image.
	imageServerAddress := fmt.Sprintf("%s:%d",
		*imageServerHostname, *imageServerPortNum)
	imgChannel := getImageChannel(imageServerAddress, imageName, timeoutTime)
	subObj := lib.Sub{
		Hostname:       *subHostname,
		Client:         srpcClient,
		ComputedInodes: computedInodes}
	deleteMissingComputedFiles := true
	ignoreMissingComputedFiles := false
	if *computedFilesRoot == "" {
		subObj.ObjectGetter = nullObjectGetterType{}
		deleteMissingComputedFiles = false
		ignoreMissingComputedFiles = true
	} else {
		fs, err := scanner.ScanFileSystem(*computedFilesRoot, nil, nil, nil,
			nil, nil)
		if err != nil {
			return err
		}
		subObj.ObjectGetter = fs
		for filename, inum := range fs.FilenameToInodeTable() {
			if inode, ok := fs.InodeTable[inum].(*filesystem.RegularInode); ok {
				computedInodes[filename] = inode
			}
		}
	}
	startTime := showStart("<-imgChannel")
	imageResult := <-imgChannel
	showTimeTaken(startTime)
	logger.Printf("Background image fetch took %s\n",
		format.Duration(imageResult.duration))
	img := imageResult.image
	var err error
	if *filterFile != "" {
		img.Filter, err = filter.Load(*filterFile)
		if err != nil {
			return err
		}
	}
	if *triggersFile != "" {
		img.Triggers, err = triggers.Load(*triggersFile)
		if err != nil {
			return err
		}
	} else if *triggersString != "" {
		img.Triggers, err = triggers.Decode([]byte(*triggersString))
		if err != nil {
			return err
		}
	}
	err = pollFetchAndPush(&subObj, img, imageServerAddress, timeoutTime, false,
		logger)
	if err != nil {
		return err
	}
	if err := srpcClient.SetKeepAlivePeriod(time.Second); err != nil {
		return fmt.Errorf("error setting keep-alive period: %s", err)
	}
	var updateRequest sub.UpdateRequest
	var updateReply sub.UpdateResponse
	startTime = showStart("lib.BuildUpdateRequest()")
	if lib.BuildUpdateRequest(subObj, img, &updateRequest,
		deleteMissingComputedFiles, ignoreMissingComputedFiles, logger) {
		showBlankLine()
		return errors.New("missing computed file(s)")
	}
	showTimeTaken(startTime)
	updateRequest.ImageName = imageName
	updateRequest.Wait = true
	stopTicker := make(chan struct{}, 1)
	if !*showTimes {
		logger.Println("Starting Subd.Update()")
		go tickerLoop(stopTicker)
	}
	startTime = showStart("Subd.Update()")
	err = client.CallUpdate(srpcClient, updateRequest, &updateReply)
	stopTicker <- struct{}{}
	if err != nil {
		showBlankLine()
		return err
	}
	showTimeTaken(startTime)
	return nil
}

func getImageChannel(clientName, imageName string,
	timeoutTime time.Time) <-chan timedImageFetch {
	resultChannel := make(chan timedImageFetch, 1)
	go func() {
		startTime := time.Now()
		img, err := getImageRetry(clientName, imageName, timeoutTime)
		if err != nil {
			logger.Fatalf("Error getting image: %s\n", err)
		}
		resultChannel <- timedImageFetch{img, time.Since(startTime)}
	}()
	return resultChannel
}

func getImageRetry(clientName, imageName string,
	timeoutTime time.Time) (*image.Image, error) {
	imageSrpcClient, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return nil, err
	}
	defer imageSrpcClient.Close()
	firstTime := true
	for ; time.Now().Before(timeoutTime); time.Sleep(time.Second) {
		img, err := imgclient.GetImage(imageSrpcClient, imageName)
		if err != nil {
			return nil, err
		} else if img != nil {
			if err := img.FileSystem.RebuildInodePointers(); err != nil {
				return nil, err
			}
			img.FileSystem.InodeToFilenamesTable()
			img.FileSystem.FilenameToInodeTable()
			img.FileSystem.HashToInodesTable()
			img.FileSystem.ComputeTotalDataBytes()
			img.FileSystem.BuildEntryMap()
			return img, nil
		} else if firstTime {
			logger.Printf("Image: %s not found, will retry\n", imageName)
			firstTime = false
		}
	}
	return nil, errors.New("timed out getting image")
}

func pollFetchAndPush(subObj *lib.Sub, img *image.Image,
	imageServerAddress string, timeoutTime time.Time, singleFetch bool,
	logger log.DebugLogger) error {
	var generationCount, lastGenerationCount, lastScanCount uint64
	deleteEarly := *deleteBeforeFetch
	ignoreMissingComputedFiles := true
	pushComputedFiles := true
	if *computedFilesRoot == "" {
		ignoreMissingComputedFiles = false
		pushComputedFiles = false
	}
	logger.Println("Starting polling loop, waiting for completed scan")
	interval := time.Second
	for ; time.Now().Before(timeoutTime); time.Sleep(interval) {
		var pollReply sub.PollResponse
		if err := client.BoostCpuLimit(subObj.Client); err != nil {
			return err
		}
		if err := pollAndBuildPointers(subObj.Client, &generationCount,
			&pollReply); err != nil {
			return err
		}
		if pollReply.FileSystem == nil {
			if interval < 5*time.Second {
				interval += 200 * time.Millisecond
			}
		} else {
			interval = time.Second
		}
		if !*showTimes {
			if pollReply.FileSystem == nil {
				fmt.Fprintf(os.Stderr, ".")
			} else {
				fmt.Fprintln(os.Stderr)
			}
		}
		if pollReply.GenerationCount != lastGenerationCount ||
			pollReply.ScanCount != lastScanCount {
			if pollReply.FileSystem == nil {
				logger.Debugf(0,
					"Poll Scan: %d, Generation: %d, cached objects: %d\n",
					pollReply.ScanCount, pollReply.GenerationCount,
					len(pollReply.ObjectCache))
			} else {
				logger.Debugf(0,
					"Poll Scan: %d, Generation: %d, FS objects: %d, cached objects: %d\n",
					pollReply.ScanCount, pollReply.GenerationCount,
					len(pollReply.FileSystem.InodeTable),
					len(pollReply.ObjectCache))
			}
		}
		lastGenerationCount = pollReply.GenerationCount
		lastScanCount = pollReply.ScanCount
		if pollReply.FileSystem == nil {
			continue
		}
		if deleteEarly {
			deleteEarly = false
			if deleteUnneededFiles(subObj.Client, pollReply.FileSystem,
				img.FileSystem, logger) {
				continue
			}
		}
		subObj.FileSystem = pollReply.FileSystem
		subObj.ObjectCache = pollReply.ObjectCache
		startTime := showStart("lib.BuildMissingLists()")
		objectsToFetch, objectsToPush := lib.BuildMissingLists(*subObj, img,
			pushComputedFiles, ignoreMissingComputedFiles, logger)
		showTimeTaken(startTime)
		if len(objectsToFetch) < 1 && len(objectsToPush) < 1 {
			return nil
		}
		if len(objectsToFetch) > 0 {
			logger.Debugf(0, "Fetch(%d)\n", len(objectsToFetch))
			startTime := showStart("Fetch()")
			err := fetchUntil(subObj, sub.FetchRequest{
				ServerAddress: imageServerAddress,
				Wait:          true,
				Hashes:        objectcache.ObjectMapToCache(objectsToFetch)},
				timeoutTime, logger)
			if err != nil {
				logger.Printf("Error calling %s:Subd.Fetch(%s): %s\n",
					subObj.Hostname, imageServerAddress, err)
				return err
			}
			showTimeTaken(startTime)
			if singleFetch {
				return nil
			}
		}
		if len(objectsToPush) > 0 {
			logger.Debugf(0, "PushObjects(%d)\n", len(objectsToPush))
			startTime := showStart("lib.PushObjects()")
			err := lib.PushObjects(*subObj, objectsToPush, logger)
			if err != nil {
				showBlankLine()
				return err
			}
			showTimeTaken(startTime)
		}
	}
	return errors.New("timed out fetching and pushing objects")
}

func fetchUntil(subObj *lib.Sub, request sub.FetchRequest,
	timeoutTime time.Time, logger log.DebugLogger) error {
	for ; time.Now().Before(timeoutTime); time.Sleep(time.Second) {
		stopTicker := make(chan struct{}, 1)
		if !*showTimes {
			logger.Println("Starting Subd.Fetch()")
			go tickerLoop(stopTicker)
		}
		err := subObj.Client.RequestReply("Subd.Fetch", request,
			&sub.FetchResponse{})
		stopTicker <- struct{}{}
		if err == nil {
			return nil
		}
		logger.Printf("Error calling %s:Subd.Fetch(): %s\n",
			subObj.Hostname, err)
	}
	return errors.New("timed out fetching objects")
}

func pollAndBuildPointers(srpcClient *srpc.Client, generationCount *uint64,
	pollReply *sub.PollResponse) error {
	pollRequest := sub.PollRequest{HaveGeneration: *generationCount}
	startTime := showStart("Poll()")
	err := client.CallPoll(srpcClient, pollRequest, pollReply)
	if err != nil {
		showBlankLine()
		return err
	}
	showTimeTaken(startTime)
	*generationCount = pollReply.GenerationCount
	fs := pollReply.FileSystem
	if fs == nil {
		return nil
	}
	startTime = showStart("FileSystem.RebuildInodePointers()")
	if err := fs.RebuildInodePointers(); err != nil {
		showBlankLine()
		return err
	}
	showTimeTaken(startTime)
	fs.BuildEntryMap()
	return nil
}

func showStart(operation string) time.Time {
	if *showTimes {
		logger.Print(operation, " ")
	}
	return time.Now()
}

func showTimeTaken(startTime time.Time) {
	if *showTimes {
		stopTime := time.Now()
		logger.Printf("took %s\n", format.Duration(stopTime.Sub(startTime)))
	}
}

func showBlankLine() {
	if *showTimes {
		logger.Println()
	}
}

func tickerLoop(stopTicker <-chan struct{}) {
	for {
		timer := time.NewTimer(time.Second)
		select {
		case <-timer.C:
			fmt.Fprintf(os.Stderr, ".")
		case <-stopTicker:
			if !timer.Stop() {
				<-timer.C
			}
			fmt.Fprintln(os.Stderr)
			return
		}
	}
}

func deleteUnneededFiles(srpcClient *srpc.Client, subFS *filesystem.FileSystem,
	imgFS *filesystem.FileSystem, logger log.DebugLogger) bool {
	startTime := showStart("compute early files to delete")
	pathsToDelete := make([]string, 0)
	imgHashToInodesTable := imgFS.HashToInodesTable()
	imgFilenameToInodeTable := imgFS.FilenameToInodeTable()
	for pathname, inum := range subFS.FilenameToInodeTable() {
		if inode, ok := subFS.InodeTable[inum].(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				if _, ok := imgHashToInodesTable[inode.Hash]; !ok {
					pathsToDelete = append(pathsToDelete, pathname)
				}
			} else {
				if _, ok := imgFilenameToInodeTable[pathname]; !ok {
					pathsToDelete = append(pathsToDelete, pathname)
				}
			}
		}
	}
	showTimeTaken(startTime)
	if len(pathsToDelete) < 1 {
		return false
	}
	updateRequest := sub.UpdateRequest{
		Wait:          true,
		PathsToDelete: pathsToDelete}
	var updateReply sub.UpdateResponse
	startTime = showStart("Subd.Update() for early files to delete")
	err := client.CallUpdate(srpcClient, updateRequest, &updateReply)
	showTimeTaken(startTime)
	if err != nil {
		logger.Println(err)
	}
	return true
}
