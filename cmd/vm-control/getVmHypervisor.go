package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func getVmHypervisorSubcommand(args []string, logger log.DebugLogger) error {
	if err := getVmHypervisor(args[0], logger); err != nil {
		return fmt.Errorf("error getting VM info: %s", err)
	}
	return nil
}

func getVmHypervisor(vmHostname string, logger log.DebugLogger) error {
	if _, hypervisor, err := lookupVmAndHypervisor(vmHostname); err != nil {
		return err
	} else {
		_, err := fmt.Println(hypervisor)
		return err
	}
}
