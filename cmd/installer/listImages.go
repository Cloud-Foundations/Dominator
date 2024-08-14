package main

import (
	"fmt"
	"path/filepath"
	"time"

	imageclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

func listImagesSubcommand(args []string, logger log.DebugLogger) error {
	if err := listImages(logger); err != nil {
		return fmt.Errorf("error listing images: %s", err)
	}
	return nil
}

func listImages(logger log.DebugLogger) error {
	imageServerAddress, err := readString(
		filepath.Join(*tftpDirectory, "imageserver"), false)
	if err != nil {
		return err
	}
	client, err := srpc.DialHTTP("tcp", imageServerAddress, time.Second*15)
	if err != nil {
		return err
	}
	defer client.Close()
	imageNames, err := imageclient.ListImages(client)
	if err != nil {
		return err
	}
	verstr.Sort(imageNames)
	for _, name := range imageNames {
		fmt.Println(name)
	}
	return nil
}
