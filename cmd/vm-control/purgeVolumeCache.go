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

func purgeVolumeCacheSubcommand(args []string, logger log.DebugLogger) error {
	if err := purgeVolumeCache(args[0], logger); err != nil {
		return fmt.Errorf("error purging volume cache: %s", err)
	}
	return nil
}

func purgeVolumeCache(strMaxUnrefAge string, logger log.DebugLogger) error {
	maxUnrefAge, err := time.ParseDuration(strMaxUnrefAge)
	if err != nil {
		return err
	}
	request := proto.PurgeVolumeCacheRequest{
		MaximumUnreferencedAge: maxUnrefAge,
		StorageIndex:           *storageIndex,
	}
	if *hypervisorHostname != "" {
		return purgeVolumeCacheOnHypervisor(
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
			err := purgeVolumeCacheOnHypervisor(hypervisor, request, hLogger)
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

func purgeVolumeCacheOnHypervisor(hypervisor string,
	request proto.PurgeVolumeCacheRequest, logger log.DebugLogger) error {
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	response, err := hyperclient.PurgeVolumeCache(client, request)
	if err != nil {
		return err
	}
	logger.Printf("Purged %s, remaining: %s\n",
		format.FormatBytes(response.BytesPurged),
		format.FormatBytes(response.BytesStillUnreferenced))
	return nil
}
