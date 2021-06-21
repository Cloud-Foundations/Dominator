package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/dom/lib"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func fetchImageSubcommand(args []string, logger log.DebugLogger) error {
	startTime := showStart("getSubClient()")
	srpcClient := getSubClientRetry(logger)
	defer srpcClient.Close()
	showTimeTaken(startTime)
	if err := fetchImage(srpcClient, args[0]); err != nil {
		return fmt.Errorf("error fetching image: %s: %s", args[0], err)
	}
	return nil
}

func fetchImage(srpcClient *srpc.Client, imageName string) error {
	imageServerAddress := fmt.Sprintf("%s:%d",
		*imageServerHostname, *imageServerPortNum)
	img, err := getImageRetry(imageServerAddress, imageName, timeoutTime)
	if err != nil {
		logger.Fatalf("Error getting image: %s\n", err)
	}
	subObj := lib.Sub{
		Hostname: *subHostname,
		Client:   srpcClient,
	}
	return pollFetchAndPush(&subObj, img, imageServerAddress, timeoutTime, true,
		logger)
}
