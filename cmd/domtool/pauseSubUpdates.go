package main

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func pauseSubUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := pauseSubUpdates(client, args[0], args[1]); err != nil {
		return fmt.Errorf("error pausing updates: %s", err)
	}
	return nil
}

func pauseSubUpdates(client srpc.ClientI, hostname, reason string) error {
	if *pauseDuration < time.Minute {
		return errors.New("cannot pause updates: duration under one minute")
	}
	if reason == "" {
		return errors.New("cannot pause updates: no reason given")
	}
	request := mdbserver.PauseUpdatesRequest{
		Hostname: hostname,
		Reason:   reason,
		Remove:   *removePaused,
		Until:    time.Now().Add(*pauseDuration),
	}
	var reply mdbserver.PauseUpdatesResponse
	err := client.RequestReply("MdbServer.PauseUpdates", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
