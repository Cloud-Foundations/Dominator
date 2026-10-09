package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
)

func fetchImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := fetchImage(logger); err != nil {
		return fmt.Errorf("error fetching image: %s", err)
	}
	return nil
}

func fetchImage(logger log.DebugLogger) error {
	if *hypervisorHostname != "" {
		return fetchImageOnHypervisor(
			fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			*imageName, logger)
	}
	if *fleetManagerHostname == "" {
		return fmt.Errorf("no Hypervisor or FleetManager specified")
	}
	hypervisorAddresses, err := getHypervisorsList()
	if err != nil {
		return err
	}
	var someFailed bool
	var wg sync.WaitGroup
	for _, hypervisor := range hypervisorAddresses {
		hypervisorHostname := strings.Split(hypervisor, ":")[0]
		hLogger := prefixlogger.New(hypervisorHostname+": ", logger)
		wg.Go(func() {
			err := fetchImageOnHypervisor(hypervisor, *imageName, hLogger)
			if err != nil {
				hLogger.Println(err)
				someFailed = true
			}
		})
	}
	wg.Wait()
	if someFailed {
		return errors.New("some Hypervisors failed to fetch")
	}
	return nil
}

func fetchImageOnHypervisor(hypervisor string, searchName string,
	logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	response, err := hyperclient.FetchImage(client, searchName, *imageTimeout)
	if err != nil {
		return err
	}
	logger.Printf(
		"Fetched: %s in %s, rebuilt inodes in: %s, downloaded %d/%d objects, %s/%s in: %s\n",
		response.ImageName,
		format.Duration(response.ImageFetchTime),
		format.Duration(response.ImageRebuildTime),
		response.DownloadedObjects,
		response.ImageObjects,
		format.FormatBytes(response.DownloadedBytes),
		format.FormatBytes(response.ImageBytes),
		format.Duration(response.DownloadTime))
	return nil
}
