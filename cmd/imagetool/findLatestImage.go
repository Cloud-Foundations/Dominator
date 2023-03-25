package main

import (
	"errors"
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func findLatestImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := findLatestImage(args[0]); err != nil {
		return fmt.Errorf("error finding latest image: %s", err)
	}
	return nil
}

func findLatestImage(dirname string) error {
	imageSClient, _ := getClients()
	imageName, err := client.FindLatestImageReq(imageSClient,
		proto.FindLatestImageRequest{
			BuildCommitId:        *buildCommitId,
			DirectoryName:        dirname,
			IgnoreExpiringImages: *ignoreExpiring,
		})
	if err != nil {
		return err
	}
	if imageName == "" {
		return errors.New("no image found")
	}
	fmt.Println(imageName)
	return nil
}
