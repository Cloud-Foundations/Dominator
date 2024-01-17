package main

import (
	"fmt"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getCapacitySubcommand(args []string, logger log.DebugLogger) error {
	err := getCapacity(logger)
	if err != nil {
		return fmt.Errorf("error getting capacity: %s", err)
	}
	return nil
}

func getCapacity(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("hypervisorHostname unspecified")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	reply, err := hyperclient.GetCapacity(client)
	if err != nil {
		return err
	}
	return json.WriteWithIndent(os.Stdout, "    ", reply)
}
