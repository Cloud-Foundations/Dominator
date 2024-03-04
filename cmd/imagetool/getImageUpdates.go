package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func getImageUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	imageClient, _ := getClients()
	if err := getImageUpdates(imageClient); err != nil {
		return fmt.Errorf("error getting image updates: %s", err)
	}
	return nil
}

func getImageUpdates(imageSClient *srpc.Client) error {
	conn, err := imageSClient.Call("ImageServer.GetImageUpdates")
	if err != nil {
		return err
	}
	var initialListReceived bool
	for {
		var imageUpdate proto.ImageUpdate
		if err := conn.Decode(&imageUpdate); err != nil {
			if err == io.EOF {
				return err
			}
			return errors.New("decode err: " + err.Error())
		}
		switch imageUpdate.Operation {
		case proto.OperationAddImage:
			if imageUpdate.Name == "" { // Initial list has been sent.
				fmt.Println("INITIAL list received")
				initialListReceived = true
			} else if initialListReceived {
				fmt.Printf("ADD: %s\n", imageUpdate.Name)
			} else {
				fmt.Printf("INIT: %s\n", imageUpdate.Name)
			}
		case proto.OperationDeleteImage:
			fmt.Printf("DELETE: %s\n", imageUpdate.Name)
		case proto.OperationMakeDirectory:
			directory := imageUpdate.Directory
			if directory == nil {
				return errors.New("nil imageUpdate.Directory")
			}
			if initialListReceived {
				fmt.Printf("MKDIR: %s\n", directory.Name)
			} else {
				fmt.Printf("DIR: %s\n", directory.Name)
			}
		}
	}
}
