package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func showImageMetadataSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageMetadata(args[0]); err != nil {
		return fmt.Errorf("error showing image metadata: %s", err)
	}
	return nil
}

func showImageMetadata(imageName string) error {
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
	return json.WriteWithIndent(os.Stdout, "    ", reply.Image)
}
