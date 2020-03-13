package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func showImageFilterSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageFilter(args[0]); err != nil {
		return fmt.Errorf("Error showing image filter: %s", err)
	}
	return nil
}

func showImageFilter(imageName string) error {
	imageSClient, _ := getClients()
	request := imageserver.GetImageRequest{
		ImageName:        imageName,
		IgnoreFilesystem: true,
		Timeout:          *timeout,
	}
	var reply imageserver.GetImageResponse
	err := imageSClient.RequestReply("ImageServer.GetImage", request, &reply)
	if err != nil {
		return err
	}
	if reply.Image == nil {
		return fmt.Errorf("no image")
	}
	if reply.Image.Filter == nil {
		return fmt.Errorf("no filter")
	}
	for _, line := range reply.Image.Filter.FilterLines {
		fmt.Println(line)
	}
	return nil
}
