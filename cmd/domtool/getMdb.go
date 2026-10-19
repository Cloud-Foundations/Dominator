package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
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
	machines, err := mdbclient.GetMdb(client)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", machines)
}
