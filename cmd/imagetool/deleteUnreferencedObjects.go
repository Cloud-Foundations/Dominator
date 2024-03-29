package main

import (
	"fmt"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func deleteUnreferencedObjectsSubcommand(args []string,
	logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	percentage, err := strconv.ParseUint(args[0], 10, 8)
	if err != nil {
		return fmt.Errorf("error parsing percentage: %s", err)
	}
	bytes, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing bytes: %s", err)
	}
	if err := client.DeleteUnreferencedObjects(imageSClient, uint8(percentage),
		bytes); err != nil {
		return fmt.Errorf("error deleting unreferenced objects: %s", err)
	}
	return nil
}
