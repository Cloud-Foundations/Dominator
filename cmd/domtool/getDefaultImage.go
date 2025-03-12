package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getDefaultImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := getDefaultImage(getClient()); err != nil {
		return fmt.Errorf("error getting default image: %s", err)
	}
	return nil
}

func getDefaultImage(client *srpc.Client) error {
	imageName, err := domclient.GetDefaultImage(client)
	if err != nil {
		return err
	}
	if imageName != "" {
		fmt.Println(imageName)
	}
	return nil
}
