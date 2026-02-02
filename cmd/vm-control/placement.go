package main

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type placementMessage struct {
	Hypervisors []fm_proto.Hypervisor `json:",omitempty"`
	VmInfo      hyper_proto.VmInfo
}

type placementType uint

const (
	placementChoiceAny = iota
	placementChoiceCommand
	placementChoiceEmptiest
	placmentChoiceFullest
	placementChoiceRandom

	placementTypeUnknown = "UNKNOWN placementType"
)

var (
	placementTypeToText = map[placementType]string{
		placementChoiceAny:      "any",
		placementChoiceCommand:  "command",
		placementChoiceEmptiest: "emptiest",
		placmentChoiceFullest:   "fullest",
		placementChoiceRandom:   "random",
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

// Returns true if the Hypervisor has capacity.
func checkHypervisorCapacity(h fm_proto.Hypervisor,
	vmInfo hyper_proto.VmInfo) bool {
	if vmInfo.MemoryInMiB+h.AllocatedMemory > h.MemoryInMiB {
		return false
	}
	if uint64(vmInfo.MilliCPUs)+h.AllocatedMilliCPUs > uint64(h.NumCPUs*1000) {
		return false
	}
	if h.AvailableMemory > 0 &&
		vmInfo.MemoryInMiB >= h.AvailableMemory {
		return false
	}
	var totalVolumeSize uint64
	for _, volume := range vmInfo.Volumes {
		totalVolumeSize += volume.EffectiveSize()
	}
	if totalVolumeSize+h.AllocatedVolumeBytes > h.TotalVolumeBytes {
		return false
	}
	if !checkHypervisorSubnetCapacity(h, vmInfo.SubnetId) {
		return false
	}
	for _, subnetId := range vmInfo.SecondarySubnetIDs {
		if !checkHypervisorSubnetCapacity(h, subnetId) {
			return false
		}
	}
	return true
}

// Returns true if the subnet on the Hypervisor has capacity.
func checkHypervisorSubnetCapacity(h fm_proto.Hypervisor, subnet string) bool {
	if numFree, ok := h.NumFreeAddresses[subnet]; ok && numFree < 1 {
		return false
	}
	return true
}

// Returns true if [i] has less free CPU than [j].
func compareCPU(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return freeCPU(hypervisors[i]) < freeCPU(hypervisors[j])
}

// Returns true if [i] has less free memory than [j].
func compareMemory(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return freeMemory(hypervisors[i]) < freeMemory(hypervisors[j])
}

// Returns true if [i] has less free storage space than [j].
func compareStorage(hypervisors []fm_proto.Hypervisor, i, j int) bool {
	return freeStorage(hypervisors[i]) < freeStorage(hypervisors[j])
}

func findHypervisorsWithCapacity(inputHypervisors []fm_proto.Hypervisor,
	vmInfo hyper_proto.VmInfo) []fm_proto.Hypervisor {
	outputHypervisors := make([]fm_proto.Hypervisor, 0, len(inputHypervisors))
	for _, h := range inputHypervisors {
		if !checkHypervisorCapacity(h, vmInfo) {
			continue
		}
		outputHypervisors = append(outputHypervisors, h)
	}
	return outputHypervisors
}

// Returns the number of free milliCPUs on the Hypervisor.
func freeCPU(hypervisor fm_proto.Hypervisor) uint64 {
	return uint64(hypervisor.NumCPUs)*1000 - hypervisor.AllocatedMilliCPUs
}

// Returns the number of free MiB of memory on the Hypervisor.
func freeMemory(hypervisor fm_proto.Hypervisor) uint64 {
	return hypervisor.MemoryInMiB - hypervisor.AllocatedMemory
}

// Returns the number of free bytes of storage on the Hypervisor.
func freeStorage(hypervisor fm_proto.Hypervisor) uint64 {
	return hypervisor.TotalVolumeBytes - hypervisor.AllocatedVolumeBytes
}

// getHypervisorAddress returns the Hypervisor address where the VM may be
// created.
func getHypervisorAddress(vmInfo hyper_proto.VmInfo, logger log.DebugLogger) (
	string, error) {
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
	if placement == placementChoiceAny { // Really dumb placement.
		return selectAnyHypervisor(client)
	}
	request := fm_proto.GetHypervisorsInLocationRequest{
		HypervisorTagsToMatch: hypervisorTagsToMatch,
		IncludeVMs:            placement == placementChoiceCommand,
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
	hypervisor, err := selectHypervisor(client, hypervisors, vmInfo, logger)
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

func selectHypervisor(client *srpc.Client, hypervisors []fm_proto.Hypervisor,
	vmInfo hyper_proto.VmInfo,
	logger log.DebugLogger) (*fm_proto.Hypervisor, error) {
	numHyper := len(hypervisors)
	if numHyper < 1 {
		return nil, errors.New("no Hypervisors in location with capacity")
	} else if numHyper < 2 {
		return &hypervisors[0], nil
	}
	switch placement {
	case placementChoiceCommand:
		return selectHypervisorUsingCommand(hypervisors, vmInfo)
	case placementChoiceEmptiest:
		sortHypervisors(hypervisors, logger)
		return &hypervisors[len(hypervisors)-1], nil
	case placmentChoiceFullest:
		sortHypervisors(hypervisors, logger)
		return &hypervisors[0], nil
	case placementChoiceRandom:
		return &hypervisors[rand.Intn(numHyper)], nil
	}
	return nil, errors.New(placementTypeUnknown)
}

func selectHypervisorUsingCommand(hypervisors []fm_proto.Hypervisor,
	vmInfo hyper_proto.VmInfo) (*fm_proto.Hypervisor, error) {
	if *placementCommand == "" {
		return nil, errors.New("no placementCommand")
	}
	cmd := exec.Command(*placementCommand)
	buffer := &bytes.Buffer{}
	writer, err := cmd.StdinPipe()
	defer writer.Close()
	cmd.Stdout = buffer
	cmd.Stderr = os.Stderr
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	msg := placementMessage{hypervisors, vmInfo}
	if err := json.WriteWithIndent(writer, "    ", msg); err != nil {
		return nil, err
	}
	writer.Close()
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	output := string(bytes.TrimSpace(buffer.Bytes()))
	if output == "" {
		return nil, errors.New("no output from command")
	}
	var h fm_proto.Hypervisor
	h.Hostname = output
	return &h, nil
}

// Returns true if [i] has less free capacity than [j].
func sortHypervisors(hypervisors []fm_proto.Hypervisor,
	logger log.DebugLogger) {
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareStorage(hypervisors, i, j)
	})
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareMemory(hypervisors, i, j)
	})
	sort.SliceStable(hypervisors, func(i, j int) bool {
		return compareCPU(hypervisors, i, j)
	})
	logger.Debugln(2, "Sorted Hypervisors (emptiest to fullest):")
	for _, hypervisor := range hypervisors {
		logger.Debugf(2, "  %s: free CPU: %d, memory: %d MiB, storage: %s\n",
			hypervisor.Hostname,
			freeCPU(hypervisor),
			freeMemory(hypervisor),
			format.FormatBytes(freeStorage(hypervisor)),
		)
	}
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
