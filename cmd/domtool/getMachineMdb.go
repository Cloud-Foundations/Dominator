package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
)

func getMachineMdbSubcommand(args []string, logger log.DebugLogger) error {
	if machine, err := getMachineMdb(args[0]); err != nil {
		return fmt.Errorf("error getting MDB: %s", err)
	} else {
		return json.WriteWithIndent(os.Stdout, "    ", machine)
	}
}

func getMachineMdb(hostname string) (mdb.Machine, error) {
	client, err := getMdbdClient()
	if err != nil {
		return mdb.Machine{}, err
	}
	defer client.Close()
	reply, err := mdbclient.GetMachine(client, hostname)
	if err != nil {
		return mdb.Machine{}, err
	}
	return reply.Machine, nil
}
