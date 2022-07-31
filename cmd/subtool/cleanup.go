package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	"github.com/Cloud-Foundations/Dominator/sub/client"
)

func cleanupSubcommand(args []string, logger log.DebugLogger) error {
	srpcClient := getSubClient(logger)
	defer srpcClient.Close()
	if err := cleanup(srpcClient, 0, false); err != nil {
		return fmt.Errorf("error cleaning up: %s", err)
	}
	return nil
}

func cleanup(srpcClient *srpc.Client, haveGeneration uint64,
	alwaysCleanup bool) error {
	request := sub.PollRequest{
		HaveGeneration: haveGeneration,
	}
	var reply sub.PollResponse
	if err := client.CallPoll(srpcClient, request, &reply); err != nil {
		return err
	}
	if len(reply.ObjectCache) < 1 && !alwaysCleanup {
		return nil
	}
	logger.Printf("Deleting: %d objects\n", len(reply.ObjectCache))
	return client.Cleanup(srpcClient, reply.ObjectCache)
}
