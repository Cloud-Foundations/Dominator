package main

import (
	"fmt"
	"os"
	"sort"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func getVmInfosSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmInfos(logger); err != nil {
		return fmt.Errorf("error getting VM infos: %s", err)
	}
	return nil
}

func getVmInfos(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return fmt.Errorf("hypervisorHostname not specified")
	}
	return getVmInfosOnHypervisor(
		fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
		logger)
}

func getVmInfosOnHypervisor(hypervisor string, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	vmInfos, err := hyperclient.GetVmInfos(client,
		hyper_proto.GetVmInfosRequest{
			OwnerGroups:   ownerGroups,
			OwnerUsers:    ownerUsers,
			VmTagsToMatch: vmTagsToMatch,
		})
	if err != nil {
		return err
	}
	sort.Slice(vmInfos, func(i, j int) bool {
		return verstr.Less(vmInfos[i].Address.IpAddress.String(),
			vmInfos[j].Address.IpAddress.String())
	})
	return json.WriteWithIndent(os.Stdout, "    ", vmInfos)
}
