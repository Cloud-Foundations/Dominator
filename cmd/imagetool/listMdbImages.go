package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
)

func listMdbImagesSubcommand(args []string, logger log.DebugLogger) error {
	mdbdClient, err := dialMdbd()
	if err != nil {
		return err
	}
	if err := listMdbImages(mdbdClient); err != nil {
		return fmt.Errorf("error listing MDB images: %s", err)
	}
	return nil
}

func listMdbImages(mdbdClient *srpc.Client) error {
	reply, err := mdbclient.ListImages(mdbdClient)
	if err != nil {
		return err
	}
	imageNames := make(map[string]struct{},
		len(reply.PlannedImages)+len(reply.RequiredImages))
	for _, imageName := range reply.PlannedImages {
		imageNames[imageName] = struct{}{}
	}
	for _, imageName := range reply.RequiredImages {
		imageNames[imageName] = struct{}{}
	}
	names := stringutil.ConvertMapKeysToList(imageNames, false)
	verstr.Sort(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
