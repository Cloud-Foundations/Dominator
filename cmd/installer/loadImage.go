package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

func loadImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := loadImage(args[0], args[1], logger); err != nil {
		return fmt.Errorf("error loading image: %s", err)
	}
	return nil
}

func loadImage(imageName, rootDir string, logger log.DebugLogger) error {
	imageName, img, client, err := getImage(imageName, logger)
	if err != nil {
		return err
	}
	if client != nil {
		defer client.Close()
	}
	if img == nil {
		return fmt.Errorf("image: %s does not exist", imageName)
	}
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return err
	}
	objClient := objectclient.AttachObjectClient(client)
	defer objClient.Close()
	return unpackAndMount(rootDir, img.FileSystem, objClient, true, logger)
}
