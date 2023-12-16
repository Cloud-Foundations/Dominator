package main

import (
	"fmt"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func listVolumeDirectoriesSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := listVolumeDirectories(logger); err != nil {
		return fmt.Errorf("error getting volume directories: %s", err)
	}
	return nil
}

func listVolumeDirectories(logger log.DebugLogger) error {
	if *hypervisorHostname == "" {
		return errors.New("hypervisorHostname no specified")
	}
	clientName := fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum)
	client, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	volumeDirectories, err := hyperclient.ListVolumeDirectories(client, false)
	if err != nil {
		return err
	}
	for _, volumeDirectory := range volumeDirectories {
		fmt.Println(volumeDirectory)
	}
	return nil
}
