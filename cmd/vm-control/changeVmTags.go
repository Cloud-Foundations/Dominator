package main

import (
	"fmt"
	"net"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func changeVmTagsSubcommand(args []string, logger log.DebugLogger) error {
	if err := changeVmTags(args[0], logger); err != nil {
		return fmt.Errorf("error changing VM tags: %s", err)
	}
	return nil
}

func changeVmTags(vmHostname string, logger log.DebugLogger) error {
	checkTags(logger)
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return changeVmTagsOnHypervisor(hypervisor, vmIP, logger)
	}
}

func changeVmTagsOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	if _, ok := vmTags[""]; ok {
		return hyperclient.ChangeVmTags(client, ipAddr, vmTags)
	}
	if _, ok := vmTags["*"]; ok {
		return hyperclient.ChangeVmTags(client, ipAddr, vmTags)
	}
	vmInfo, err := hyperclient.GetVmInfo(client, ipAddr)
	if err != nil {
		return err
	}
	if len(vmInfo.Tags) < 1 {
		return hyperclient.ChangeVmTags(client, ipAddr, vmTags)
	}
	vmInfo.Tags.Merge(vmTags)
	for key, value := range vmInfo.Tags {
		if value == "" {
			delete(vmInfo.Tags, key)
		}
	}
	return hyperclient.ChangeVmTags(client, ipAddr, vmInfo.Tags)
}
