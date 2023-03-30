package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func makeDirectorySubcommand(args []string, logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	if err := client.MakeDirectory(imageSClient, args[0]); err != nil {
		return fmt.Errorf("error creating directory: %s", err)
	}
	return nil
}
