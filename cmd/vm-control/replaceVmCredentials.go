package main

import (
	"fmt"
	"io/ioutil"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func replaceVmCredentialsSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := replaceVmCredentials(args[0], logger); err != nil {
		return fmt.Errorf("error replacing VM credentials: %s", err)
	}
	return nil
}

func replaceVmCredentials(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return replaceVmCredentialsOnHypervisor(hypervisor, vmIP, logger)
	}
}

func replaceVmCredentialsOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	identityCert, err := ioutil.ReadFile(*identityCertFile)
	if err != nil {
		return err
	}
	identityKey, err := ioutil.ReadFile(*identityKeyFile)
	if err != nil {
		return err
	}
	request := proto.ReplaceVmCredentialsRequest{
		IdentityCertificate: identityCert,
		IdentityKey:         identityKey,
		IpAddress:           ipAddr,
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var response proto.ReplaceVmCredentialsResponse
	err = client.RequestReply("Hypervisor.ReplaceVmCredentials", request,
		&response)
	if err != nil {
		return err
	}
	return errors.New(response.Error)
}
