package main

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type placementType uint

const (
	placementAny = iota
	placementRandom
	placementEmptiest
	placmentFullest

	placementTypeUnknown = "UNKNOWN placementType"
)

var (
	placementTypeToText = map[placementType]string{
		placementAny:      "any",
		placementRandom:   "random",
		placementEmptiest: "emptiest",
		placmentFullest:   "fullest",
	}
	textToPlacementType map[string]placementType
)

func init() {
	rand.Seed(time.Now().Unix() + time.Now().UnixNano())
	textToPlacementType = make(map[string]placementType,
		len(placementTypeToText))
	for placementType, text := range placementTypeToText {
		textToPlacementType[text] = placementType
	}
}

// Returns true if [i] has less free CPU than [j].
func compareCPU(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return uint64(hypervisors[i].NumCPUs)*1000-
		hypervisors[i].AllocatedMilliCPUs <
		uint64(hypervisors[j].NumCPUs)*1000-hypervisors[j].AllocatedMilliCPUs
}

// Returns true if [i] has less free memory than [j].
func compareMemory(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return hypervisors[i].MemoryInMiB-hypervisors[i].AllocatedMemory <
		hypervisors[j].MemoryInMiB-hypervisors[j].AllocatedMemory
}

// Returns true if [i] has less free storage space than [j].
func compareStorage(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return hypervisors[i].TotalVolumeBytes-hypervisors[i].AllocatedVolumeBytes <
		hypervisors[j].TotalVolumeBytes-hypervisors[j].AllocatedVolumeBytes
}

func findHypervisorsWithCapacity(inputHypervisors []fm_proto.Hypervisor,
	vmInfo hyper_proto.VmInfo) []fm_proto.Hypervisor {
	outputHypervisors := make([]fm_proto.Hypervisor, 0, len(inputHypervisors))
	for _, h := range inputHypervisors {
		if vmInfo.MemoryInMiB+h.AllocatedMemory > h.MemoryInMiB {
			continue
		}
		if uint64(vmInfo.MilliCPUs)+h.AllocatedMilliCPUs >
			uint64(h.NumCPUs*1000) {
			continue
		}
		var totalVolumeSize uint64
		for _, volume := range vmInfo.Volumes {
			totalVolumeSize += volume.Size
		}
		if totalVolumeSize+h.AllocatedVolumeBytes > h.TotalVolumeBytes {
			continue
		}
		outputHypervisors = append(outputHypervisors, h)
	}
	return outputHypervisors
}

func getHypervisorAddress(vmInfo hyper_proto.VmInfo) (string, error) {
	if *hypervisorHostname != "" {
		return fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			nil
	}
	client, err := dialFleetManager(fmt.Sprintf("%s:%d",
		*fleetManagerHostname, *fleetManagerPortNum))
	if err != nil {
		return "", err
	}
	defer client.Close()
	if *adjacentVM != "" {
		if adjacentVmIpAddr, err := lookupIP(*adjacentVM); err != nil {
			return "", err
		} else {
			return findHypervisorClient(client, adjacentVmIpAddr)
		}
	}
	if placement == placementAny { // Really dumb placement.
		return selectAnyHypervisor(client)
	}
	request := fm_proto.GetHypervisorsInLocationRequest{
		HypervisorTagsToMatch: hypervisorTagsToMatch,
		Location:              *location,
		SubnetId:              *subnetId,
	}
	var reply fm_proto.GetHypervisorsInLocationResponse
	err = client.RequestReply("FleetManager.GetHypervisorsInLocation",
		request, &reply)
	if err != nil {
		return "", err
	}
	if reply.Error != "" {
		return "", errors.New(reply.Error)
	}
	hypervisors := findHypervisorsWithCapacity(reply.Hypervisors, vmInfo)
	hypervisor, err := selectHypervisor(client, hypervisors)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d",
		hypervisor.Hostname, constants.HypervisorPortNumber), nil
}

func selectAnyHypervisor(client *srpc.Client) (string, error) {
	request := fm_proto.ListHypervisorsInLocationRequest{
		HypervisorTagsToMatch: hypervisorTagsToMatch,
		Location:              *location,
		SubnetId:              *subnetId,
	}
	var reply fm_proto.ListHypervisorsInLocationResponse
	err := client.RequestReply("FleetManager.ListHypervisorsInLocation",
		request, &reply)
	if err != nil {
		return "", err
	}
	if reply.Error != "" {
		return "", errors.New(reply.Error)
	}
	numHyper := len(reply.HypervisorAddresses)
	if numHyper < 1 {
		return "", errors.New("no active Hypervisors in location")
	} else if numHyper < 2 {
		return reply.HypervisorAddresses[0], nil
	}
	return reply.HypervisorAddresses[rand.Intn(numHyper)], nil
}

func selectHypervisor(client *srpc.Client,
	hypervisors []fm_proto.Hypervisor) (*fm_proto.Hypervisor, error) {
	numHyper := len(hypervisors)
	if numHyper < 1 {
		return nil, errors.New("no Hypervisors in location with capacity")
	} else if numHyper < 2 {
		return &hypervisors[0], nil
	}
	switch placement {
	case placementRandom:
		return &hypervisors[rand.Intn(numHyper)], nil
	case placementEmptiest:
		sortHypervisors(hypervisors)
		return &hypervisors[len(hypervisors)-1], nil
	case placmentFullest:
		sortHypervisors(hypervisors)
		return &hypervisors[0], nil
	}
	return nil, errors.New(placementTypeUnknown)
}

// Returns true if [i] has less free capacity than [j].
func sortHypervisors(hypervisors []fm_proto.Hypervisor) {
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareStorage(hypervisors, i, j)
	})
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareMemory(hypervisors, i, j)
	})
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareCPU(hypervisors, i, j)
	})
}

func (p *placementType) Set(value string) error {
	if val, ok := textToPlacementType[value]; !ok {
		return errors.New(placementTypeUnknown)
	} else {
		*p = val
		return nil
	}
}

func (p placementType) String() string {
	if str, ok := placementTypeToText[p]; !ok {
		return placementTypeUnknown
	} else {
		return str
	}
}
