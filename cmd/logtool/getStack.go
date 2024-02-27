package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/logger"
)

func getStackTraceSubcommand(args []string, logger log.DebugLogger) error {
	clients, _, err := dial(false)
	if err != nil {
		return err
	}
	if err := getStackTrace(clients[0]); err != nil {
		return fmt.Errorf("error getting stack trace: %s", err)
	}
	return nil
}

func getStackTrace(client *srpc.Client) error {
	var reply proto.GetStackTraceResponse
	err := client.RequestReply("Logger.GetStackTrace",
		proto.GetStackTraceRequest{}, &reply)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(reply.Data)
	return err
}
