package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	"github.com/Cloud-Foundations/Dominator/proto/imageserver"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func listImagesNotInMdbSubcommand(args []string, logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	mdbdSClient, err := dialMdbd()
	if err != nil {
		return err
	}
	if err := listImagesNotInMdb(imageSClient, mdbdSClient); err != nil {
		return fmt.Errorf("error listing images not in MDB: %s", err)
	}
	return nil
}

func listImagesNotInMdb(imageSClient, mdbdSClient *srpc.Client) error {
	allImageNames, err := client.ListSelectedImages(imageSClient,
		imageserver.ListSelectedImagesRequest{
			IgnoreExpiringImages: *ignoreExpiring,
		})
	if err != nil {
		return err
	}
	request := mdbserver.ListImagesRequest{}
	var reply mdbserver.ListImagesResponse
	err = mdbdSClient.RequestReply("MdbServer.ListImages", request, &reply)
	if err != nil {
		return err
	}
	mdbImageNames := make(map[string]struct{},
		len(reply.PlannedImages)+len(reply.RequiredImages))
	for _, imageName := range reply.PlannedImages {
		mdbImageNames[imageName] = struct{}{}
	}
	for _, imageName := range reply.RequiredImages {
		mdbImageNames[imageName] = struct{}{}
	}
	var unrefImageNames []string
	for _, imageName := range allImageNames {
		if _, ok := mdbImageNames[imageName]; !ok {
			unrefImageNames = append(unrefImageNames, imageName)
		}
	}
	verstr.Sort(unrefImageNames)
	for _, name := range unrefImageNames {
		fmt.Println(name)
	}
	return nil
}
