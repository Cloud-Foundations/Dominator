package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/client"
)

func pushMissingObjectsSubcommand(args []string, logger log.DebugLogger) error {
	srpcClient := getSubClientRetry(logger)
	defer srpcClient.Close()
	if err := pushMissingObjects(srpcClient, args[0]); err != nil {
		return fmt.Errorf("error pushing missing objects: %s: %s", args[0], err)
	}
	return nil
}

func pushMissingObjects(srpcClient *srpc.Client, imageName string) error {
	// Start querying the imageserver for the image.
	imgChannel := getImageChannel(getImageServerAddress(), imageName,
		timeoutTime)
	subObj := lib.Sub{
		Hostname: *subHostname,
		Client:   srpcClient,
	}
	pollRequest := sub.PollRequest{}
	var pollReply sub.PollResponse
	if err := client.CallPoll(srpcClient, pollRequest, &pollReply); err != nil {
		return err
	}
	fs := pollReply.FileSystem
	if fs == nil {
		return errors.New("sub not ready")
	}
	subObj.FileSystem = fs
	objSrv := objectclient.NewObjectClient(getObjectServerAddress())
	adderQueue, err := objectclient.NewObjectAdderQueue(srpcClient)
	if err != nil {
		return err
	}
	defer adderQueue.Close()
	imageResult := <-imgChannel
	img := imageResult.image
	if *filterFile != "" {
		var err error
		img.Filter, err = filter.Load(*filterFile)
		if err != nil {
			return err
		}
	}
	objectsToFetch, _ := lib.BuildMissingLists(subObj, img, false, true,
		logger)
	var totalBytes uint64
	for _, size := range objectsToFetch {
		totalBytes += size
	}
	logger.Printf("Pushing %d objects (%s)\n",
		len(objectsToFetch), format.FormatBytes(totalBytes))
	hashes := objectcache.ObjectMapToCache(objectsToFetch)
	objectsReader, err := objSrv.GetObjects(hashes)
	if err != nil {
		return err
	}
	defer objectsReader.Close()
	startTime := time.Now()
	for _, hashVal := range hashes {
		fmt.Printf("%x\n", hashVal)
		length, reader, err := objectsReader.NextObject()
		if err != nil {
			return err
		}
		_, err = adderQueue.Add(reader, length)
		reader.Close()
		if err != nil {
			return err
		}
	}
	if err := adderQueue.Close(); err != nil {
		return err
	}
	duration := time.Since(startTime)
	speed := uint64(float64(totalBytes) / duration.Seconds())
	logger.Printf("Pushed %d objects (%s) in %s (%s/s)\n",
		len(objectsToFetch), format.FormatBytes(totalBytes),
		format.Duration(duration), format.FormatBytes(speed))
	return nil
}
