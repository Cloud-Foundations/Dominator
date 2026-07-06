package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

func getAllocationUpdatesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := getUpdates(args[0], logger); err != nil {
		return fmt.Errorf("error getting allocation updates: %s", err)
	}
	return nil
}

func getUpdates(startPos string, logger log.DebugLogger) error {
	if *allocationManagerHostname == "" {
		return fmt.Errorf("no allocationManagerHostname specified")
	}
	startingPosition, err := strconv.ParseUint(startPos, 10, 64)
	if err != nil {
		return err
	}
	address := fmt.Sprintf("%s:%d",
		*allocationManagerHostname, *allocationManagerPortNum)
	allocatorClient, err := dialAllocationManager(address)
	if err != nil {
		return err
	}
	defer allocatorClient.Close()
	watchConn, err := allocatorClient.Call("FleetManager.GetAllocationUpdates")
	if err != nil {
		return fmt.Errorf("error calling FleetManager.GetAllocationUpdates: %s",
			err)
	}
	defer watchConn.Close()
	err = watchConn.Encode(fm_proto.GetAllocationUpdatesRequest{
		IncludeRequests: *includeAllocationRequests,
		Position:        startingPosition,
	})
	if err != nil {
		return err
	}
	if err := watchConn.Flush(); err != nil {
		return err
	}
	for {
		var response fm_proto.AllocationUpdate
		if err := watchConn.Decode(&response); err != nil {
			return fmt.Errorf("error decoding AllocationUpdate response: %s",
				err)
		}
		if response.Error != "" {
			return fmt.Errorf("error decoding AllocationUpdate response: %s",
				response.Error)
		}
		err = json.WriteWithIndent(os.Stdout, "    ", response)
		if err != nil {
			return err
		}
	}
}
