package main

import (
	"fmt"
	"sync"
	"time"

	fmclient "github.com/Cloud-Foundations/Dominator/fleetmanager/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func createVMsSubcommand(args []string,
	logger log.DebugLogger) error {
	if err := createVMs(logger); err != nil {
		return fmt.Errorf("error creating VMs: %s", err)
	}
	return nil
}

func allocateAndCreateVMs(requests []hyper_proto.CreateVmRequest,
	vmInfo *hyper_proto.VmInfo, logger log.DebugLogger) error {
	volumes := make([]fm_proto.VolumeSpecification, 0, len(vmInfo.Volumes))
	for _, volume := range vmInfo.Volumes {
		volumes = append(volumes, fm_proto.VolumeSpecification{
			Size: types.Bytes(volume.Size),
		})
	}
	vmSpecs := make([]fm_proto.VmAllocationSpecification, 0, len(requests))
	for _, request := range requests {
		vmSpecs = append(vmSpecs,
			fm_proto.VmAllocationSpecification{
				HypervisorArchitecture: hypervisorArchitectureToMatch,
				HypervisorTagsToMatch:  hypervisorTagsToMatch,
				Location:               *location,
				MemoryInMiB:            request.MemoryInMiB,
				MilliCPUs:              request.MilliCPUs,
				NetworkInterfaces: []fm_proto.NetworkInterfaceSpecification{
					{
						SubnetId: request.SubnetId,
					},
				},
				Volumes: volumes,
			})
	}
	allocateRequest := fm_proto.AllocateRequest{
		VMs: vmSpecs,
	}
	if *allocateTimeout > 0 {
		allocateRequest.Deadline = time.Now().Add(*allocateTimeout)
	}
	address := fmt.Sprintf("%s:%d",
		*allocationManagerHostname, *allocationManagerPortNum)
	allocatorClient, err := dialAllocationManager(address)
	if err != nil {
		return err
	}
	defer allocatorClient.Close()
	allocateResponse, err := fmclient.Allocate(allocatorClient, allocateRequest)
	if err != nil {
		return err
	}
	logger.Printf("RequestId: %s\n", allocateResponse.RequestId)
	for _, request := range requests {
		request.Tags["AllocationRequestId"] =
			string(allocateResponse.RequestId)
	}
	watchConn, err := allocatorClient.Call("FleetManager.GetAllocationUpdates")
	if err != nil {
		return fmt.Errorf("error calling FleetManager.GetAllocationUpdates: %s",
			err)
	}
	doClose := true
	defer func() {
		if doClose {
			watchConn.Close()
		}
	}()
	err = watchConn.Encode(fm_proto.GetAllocationUpdatesRequest{
		Position:       allocateResponse.UpdatePosition,
		UntilRequestId: allocateResponse.RequestId,
	})
	if err != nil {
		return err
	}
	if err := watchConn.Flush(); err != nil {
		return err
	}
	watchStartTime := time.Now()
	var foundAllocation *fm_proto.Allocation
	for {
		var response fm_proto.AllocationUpdate
		if err := watchConn.Decode(&response); err != nil {
			return fmt.Errorf("error decoding AllocationUpdate response: %s",
				err)
		}
		if err := errors.New(response.Error); err != nil {
			return err
		}
		if response.RequestId != allocateResponse.RequestId {
			continue
		}
		if available := response.Available; available != nil {
			foundAllocation = available
			break
		}
		if deleted := response.Deleted; deleted != nil {
			if err := errors.New(deleted.Error); err != nil {
				return err
			}
			return fmt.Errorf("allocation request %s", deleted.Reason)
		}
	}
	doClose = false
	if err := watchConn.Close(); err != nil {
		return err
	}
	createStartTime := time.Now()
	logger.Debugf(0, "Waited %s for allocation\n",
		format.Duration(createStartTime.Sub(watchStartTime)))
	var mutex sync.Mutex
	var firstError error
	var wg sync.WaitGroup
	for index, request := range requests {
		hypervisorAddress := foundAllocation.VMs[index].HypervisorAddress
		hLogger := prefixlogger.New(hypervisorAddress+": ", logger)
		hLogger.Debugln(0, "creating VM")
		req := createVmRequest{CreateVmRequest: request}
		wg.Go(func() {
			_, err := createVmOnHypervisor(hypervisorAddress, req, hLogger)
			if err != nil {
				mutex.Lock()
				if firstError == nil {
					firstError = fmt.Errorf("%s: %s", hypervisorAddress, err)
				}
				mutex.Unlock()
			}
		})
	}
	wg.Wait()
	logger.Debugf(0, "Waited %s for creations\n",
		format.Duration(time.Since(createStartTime)))
	if firstError != nil {
		logger.Debugf(0, "cancelling allocation request: %s\n",
			allocateResponse.RequestId)
		err := fmclient.CancelAllocation(allocatorClient,
			allocateResponse.RequestId)
		if err != nil {
			logger.Println(err)
		}
	}
	return firstError
}

func createVMs(logger log.DebugLogger) error {
	var requests []hyper_proto.CreateVmRequest
	var err error
	if *numVMs < 1 && *requestFile != "" {
		err = json.ReadFromFile(*requestFile, &requests)
	} else {
		requests, err = makeCreateVMsRequest(logger)
	}
	if err != nil {
		return err
	}
	if len(requests) < 1 {
		return nil
	}
	if err := validateVmCreateRequest(&requests[0]); err != nil {
		return err
	}
	tmpVmInfo, err := approximateVolumesForCreateRequest(requests[0].VmInfo)
	if err != nil {
		return err
	}
	if *allocationManagerHostname != "" {
		return allocateAndCreateVMs(requests, tmpVmInfo, logger)
	}
	for _, request := range requests {
		req := createVmRequest{CreateVmRequest: request}
		if _, err := createOneVm(req, *tmpVmInfo, logger); err != nil {
			return err
		}
	}
	return nil
}

func makeCreateVMsRequest(logger log.DebugLogger) (
	[]hyper_proto.CreateVmRequest, error) {
	if *imageFile != "" {
		return nil, errors.New("cannot specify imageFile")
	}
	if *vmHostname == "" {
		return nil, errors.New("no hostname specified")
	}
	if *requestFile != "" {
		var requests []hyper_proto.CreateVmRequest
		if err := json.ReadFromFile(*requestFile, &requests); err != nil {
			return nil, err
		}
		return requests, nil
	}
	singleRequest, err := makeCreateVmRequest(logger)
	if err != nil {
		return nil, err
	}
	requests := make([]hyper_proto.CreateVmRequest, 0, *numVMs)
	for index := range *numVMs {
		request := singleRequest.CreateVmRequest
		request.Hostname = fmt.Sprintf(*vmHostname, index)
		request.Tags = singleRequest.Tags.Copy()
		request.Tags["Name"] = request.Hostname
		requests = append(requests, request)
	}
	return requests, nil
}
