package main

import (
	"fmt"
	"net"
	"os"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type localVmInfo struct {
	Hypervisor string
	proto.VmInfo
}

func getVmInfoSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmInfo(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM info: %s", err)
	}
	return nil
}

func getVmInfo(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return getVmInfoOnHypervisor(hypervisor, vmIP, logger)
	}
}

func getVmInfoOnHypervisor(hypervisor string, ipAddr net.IP,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	if vmInfo, err := hyperclient.GetVmInfo(client, ipAddr); err != nil {
		return err
	} else {
		return json.WriteWithIndent(os.Stdout, "    ", localVmInfo{
			Hypervisor: hypervisor,
			VmInfo:     vmInfo,
		})
	}
}
