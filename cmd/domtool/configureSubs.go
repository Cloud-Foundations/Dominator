package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

func configureSubsSubcommand(args []string, logger log.DebugLogger) error {
	if err := configureSubs(getClient()); err != nil {
		return fmt.Errorf("error setting config for subs: %s", err)
	}
	return nil
}

func configureSubs(client *srpc.Client) error {
	return domclient.ConfigureSubs(client, subproto.Configuration{
		CpuPercent:          *cpuPercent,
		NetworkSpeedPercent: *networkSpeedPercent,
		ScanExclusionList:   scanExcludeList,
		ScanSpeedPercent:    *scanSpeedPercent,
	})
}
