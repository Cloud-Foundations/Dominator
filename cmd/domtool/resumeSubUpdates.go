package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	mdbclient "github.com/Cloud-Foundations/Dominator/lib/mdb/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func resumeSubUpdatesSubcommand(args []string, logger log.DebugLogger) error {
	client, err := getMdbdClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := resumeSubUpdates(client, args[0]); err != nil {
		return fmt.Errorf("error pausing updates: %s", err)
	}
	return nil
}

func resumeSubUpdates(client srpc.ClientI, hostname string) error {
	return mdbclient.ResumeUpdates(client, hostname)
}
