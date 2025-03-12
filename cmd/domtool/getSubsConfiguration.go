package main

import (
	"fmt"

	domclient "github.com/Cloud-Foundations/Dominator/dom/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getSubsConfigurationSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := getSubsConfiguration(getClient()); err != nil {
		return fmt.Errorf("error getting config for subs: %s", err)
	}
	return nil
}

func getSubsConfiguration(client *srpc.Client) error {
	configuration, err := domclient.GetSubsConfiguration(client)
	if err != nil {
		return err
	}
	fmt.Println(configuration)
	return nil
}
