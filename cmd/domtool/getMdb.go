package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func getMdbSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := getMdb(client); err != nil {
		return fmt.Errorf("error getting MDB: %s", err)
	}
	return nil
}

func getMdb(client srpc.ClientI) error {
	request := mdbserver.GetMdbRequest{}
	var reply mdbserver.GetMdbResponse
	err := client.RequestReply("MdbServer.GetMdb", request, &reply)
	if err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", reply.Machines)
}
