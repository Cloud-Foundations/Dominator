package main

import (
	"fmt"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getIdentityProviderSubcommand(args []string,
	logger log.DebugLogger) error {
	err := getIdentityProvider(logger)
	if err != nil {
		return fmt.Errorf("error getting Identity Provider: %s", err)
	}
	return nil
}

func getIdentityProvider(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("hypervisorHostname unspecified")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	identityProvider, err := hyperclient.GetIdentityProvider(client)
	if err != nil {
		return err
	}
	if identityProvider == "" {
		return fmt.Errorf("Hypervisor has no Identity Provider")
	}
	_, err = fmt.Println(identityProvider)
	return err
}
