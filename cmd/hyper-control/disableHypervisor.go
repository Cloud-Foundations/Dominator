package main

import (
	"fmt"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func disableHypervisorSubcommand(args []string, logger log.DebugLogger) error {
	err := disableHypervisor(logger)
	if err != nil {
		return fmt.Errorf("error disabling Hypervisor: %s", err)
	}
	return nil
}

func disableHypervisor(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("hypervisorHostname no specified")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	return hyperclient.SetDisabledState(client, true)
}
