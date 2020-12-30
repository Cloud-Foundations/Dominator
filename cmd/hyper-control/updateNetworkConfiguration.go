package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/net/configurator"
)

func updateNetworkConfigurationSubcommand(args []string,
	logger log.DebugLogger) error {
	err := updateNetworkConfiguration(logger)
	if err != nil {
		return fmt.Errorf("error updating network configuration: %s", err)
	}
	return nil
}

func updateNetworkConfiguration(logger log.DebugLogger) error {
	_, interfaces, err := getUpInterfaces(logger)
	if err != nil {
		return err
	}
	info, err := getInfoForhost("")
	if err != nil {
		return err
	}
	netconf, err := configurator.Compute(info, interfaces, logger)
	if err != nil {
		return err
	}
	if changed, err := netconf.Update("/", logger); err != nil {
		return err
	} else if !changed {
		return nil
	}
	logger.Println("restarting hypervisor")
	cmd := exec.Command("service", "hypervisor", "restart")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
