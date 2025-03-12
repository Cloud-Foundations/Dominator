package main

import (
	"fmt"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func getPublicKeySubcommand(args []string, logger log.DebugLogger) error {
	err := getPublicKey(logger)
	if err != nil {
		return fmt.Errorf("error getting public key: %s", err)
	}
	return nil
}

func getPublicKey(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("hypervisorHostname unspecified")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	pubkey, err := hyperclient.GetPublicKey(client)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(pubkey); err != nil {
		return err
	}
	return nil
}
