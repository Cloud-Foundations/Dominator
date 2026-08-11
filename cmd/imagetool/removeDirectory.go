package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func removeDirectorySubcommand(args []string, logger log.DebugLogger) error {
	imageSClient, _ := getMasterClients()
	if err := client.DeleteDirectory(imageSClient, args[0]); err != nil {
		return fmt.Errorf("error deleting directory: %s", err)
	}
	return nil
}
