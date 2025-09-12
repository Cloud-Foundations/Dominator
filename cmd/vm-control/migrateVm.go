package main

import (
	"fmt"
	"net"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func migrateVmSubcommand(args []string, logger log.DebugLogger) error {
	if err := migrateVm(args[0], logger); err != nil {
		return fmt.Errorf("error migrating VM: %s", err)
	}
	return nil
}

func getVmAccessTokenClient(hypervisor *srpc.Client,
	ipAddr net.IP) ([]byte, error) {
	return hyperclient.GetVmAccessToken(hypervisor, ipAddr, time.Hour*24)
}

func migrateVm(vmHostname string, logger log.DebugLogger) error {
	if vmIP, hypervisor, err := searchVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		return migrateVmFromHypervisor(hypervisor, vmIP, logger)
	}
}

func migrateVmFromHypervisor(sourceHypervisorAddress string, vmIP net.IP,
	logger log.DebugLogger) error {
	sourceHypervisor, err := dialHypervisor(sourceHypervisorAddress)
	if err != nil {
		return err
	}
	defer sourceHypervisor.Close()
	vmInfo, err := hyperclient.GetVmInfo(sourceHypervisor, vmIP)
	if err != nil {
		return err
	} else if vmInfo.State == hyper_proto.StateMigrating {
		return errors.New("VM is migrating")
	}
	accessToken, err := getVmAccessTokenClient(sourceHypervisor, vmIP)
	if err != nil {
		return err
	}
	defer hyperclient.DiscardVmAccessToken(sourceHypervisor, vmIP, nil)
	destHypervisorAddress, err := getHypervisorAddress(vmInfo, logger)
	if err != nil {
		return err
	}
	destHypervisor, err := dialHypervisor(destHypervisorAddress)
	if err != nil {
		return err
	}
	defer destHypervisor.Close()
	logger.Debugf(0, "migrating VM to %s\n", destHypervisorAddress)
	request := hyper_proto.MigrateVmRequest{
		AccessToken:      accessToken,
		IpAddress:        vmIP,
		SkipMemoryCheck:  *skipMemoryCheck,
		SourceHypervisor: sourceHypervisorAddress,
	}
	return hyperclient.MigrateVm(destHypervisor, request, func() bool {
		return requestCommit(logger)
	},
		logger)
}

func requestCommit(logger log.DebugLogger) bool {
	userResponse, err := askForInputChoice("Commit VM",
		[]string{"commit", "abandon"})
	if err != nil {
		logger.Println(err)
		return false
	}
	switch userResponse {
	case "abandon":
	case "commit":
		return true
	default:
		logger.Printf("invalid response: %s\n", userResponse)
	}
	return false
}
