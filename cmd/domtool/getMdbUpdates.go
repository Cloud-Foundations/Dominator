package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

func getMdbUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := getMdbUpdates(client); err != nil {
		return fmt.Errorf("error getting MDB updates: %s", err)
	}
	return nil
}

func getMdbUpdates(client srpc.ClientI) error {
	return mdbclient.GetMdbUpdates(client,
		func(mdbUpdate mdbserver.MdbUpdate) error {
			return json.WriteWithIndent(os.Stdout, "    ", mdbUpdate)
		})
}
