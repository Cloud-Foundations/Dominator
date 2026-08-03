package main

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func importFsTreeSubcommand(args []string, logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	err := importFsTree(imageSClient, args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error importing FsTree: %s", err)
	}
	return nil
}

func importFsTree(imageSClient *srpc.Client,
	imdirname, treeUrl string, logger log.DebugLogger) error {
	if *expiresIn < 1 {
		return fmt.Errorf("must specify an expiry time")
	}
	response, err := client.ImportTree(imageSClient, proto.ImportTreeRequest{
		DirectoryName: imdirname,
		ExpiresAt:     time.Now().Add(*expiresIn),
		TreeUrl:       treeUrl,
	})
	if err != nil {
		return err
	}
	speed := float64(response.TreeBytesAdded) /
		response.TreesDownloadTime.Seconds()
	logger.Printf(
		"Imported %d (%s) new tree objects in: %s (%s/s)\n",
		response.TreeObjectsAdded,
		format.FormatBytes(response.TreeBytesAdded),
		format.Duration(response.TreesDownloadTime),
		format.FormatBytes(uint64(speed)))
	speed = float64(response.FileBytesAdded) /
		response.FilesDownloadTime.Seconds()
	logger.Printf(
		"Imported %d (%s) new data objects in: %s (%s/s)\n",
		response.FileObjectsAdded,
		format.FormatBytes(response.FileBytesAdded),
		format.Duration(response.FilesDownloadTime),
		format.FormatBytes(uint64(speed)))
	fmt.Println(response.ImageName)
	return nil
}
