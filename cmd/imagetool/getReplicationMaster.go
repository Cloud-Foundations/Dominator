package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getReplicationMasterSubcommand(args []string,
	logger log.DebugLogger) error {
	imageSClient, _ := getClients()
	if err := getReplicationMaster(imageSClient); err != nil {
		return fmt.Errorf("error getting replication master: %s", err)
	}
	return nil
}

func getReplicationMaster(imageSClient *srpc.Client) error {
	replicationMaster, err := client.GetReplicationMaster(imageSClient)
	if err != nil {
		return err
	}
	if len(replicationMaster) > 0 {
		fmt.Println(replicationMaster)
	}
	return nil
}
