package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func cacheVolumeSubcommand(args []string, logger log.DebugLogger) error {
	if err := cacheVolume(logger); err != nil {
		return fmt.Errorf("error caching volume: %s", err)
	}
	return nil
}

func cacheVolume(logger log.DebugLogger) error {
	request := proto.CacheVolumeRequest{
		ImageTimeout:     *imageTimeout,
		MinimumFreeBytes: uint64(minFreeBytes),
		RoundupPower:     *roundupPower,
		SearchName:       *imageName,
		StorageIndex:     *storageIndex,
	}
	if *hypervisorHostname != "" {
		return cacheVolumeOnHypervisor(
			fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			request, logger)
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
			err := cacheVolumeOnHypervisor(hypervisor, request, hLogger)
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

func cacheVolumeOnHypervisor(hypervisor string,
	request proto.CacheVolumeRequest, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	startTime := time.Now()
	response, err := hyperclient.CacheVolume(client, request)
	if err != nil {
		return err
	}
	timeTaken := time.Since(startTime)
	logger.Printf(
		"Cached: %s in %s, already cached: %v\n",
		response.ImageName, format.Duration(timeTaken), response.AlreadyCached)
	return nil
}
