package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func waitImageSubcommand(args []string, logger log.DebugLogger) error {
	if *masterImageServerHostname != "" {
		imageSClient, _ := getMasterClients()
		imageExists, err := client.CheckImage(imageSClient, args[0])
		if err != nil {
			return fmt.Errorf("error checking image on master: %s", err)
		}
		if !imageExists {
			return fmt.Errorf("image not present on master: %s",
				*masterImageServerHostname)
		}
	}
	imageSClient, _ := getClients()
	request := imageserver.GetImageRequest{
		ImageName:        args[0],
		IgnoreFilesystem: true,
		Timeout:          *timeout,
	}
	var reply imageserver.GetImageResponse
	err := imageSClient.RequestReply("ImageServer.GetImage", request, &reply)
	if err != nil {
		return err
	}
	if reply.Image != nil {
		return nil
	}
	os.Exit(1)
	panic("impossible")
}
