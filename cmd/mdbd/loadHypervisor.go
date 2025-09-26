package main

import (
	"fmt"
	"sync"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var emptyTags = make(map[string]string)

type hypervisorGeneratorType struct {
	eventChannel chan<- struct{}
	logger       log.DebugLogger
	mutex        sync.Mutex
	vms          map[string]*proto.VmInfo
}

func newHypervisorGenerator(params makeGeneratorParams) (generator, error) {
	g := &hypervisorGeneratorType{
		eventChannel: params.eventChannel,
		logger:       params.logger,
		vms:          make(map[string]*proto.VmInfo),
	}
	params.waitGroup.Add(1)
	go g.daemon(params.waitGroup)
	return g, nil
}

func (g *hypervisorGeneratorType) daemon(waitGroup *sync.WaitGroup) {
	address := fmt.Sprintf(":%d", constants.HypervisorPortNumber)
	for {
		if err := g.getUpdates(address, waitGroup); err != nil {
			g.logger.Println(err)
			time.Sleep(time.Second)
		}
		waitGroup = nil
	}
}

func (g *hypervisorGeneratorType) getUpdates(hypervisor string,
	waitGroup *sync.WaitGroup) error {
	client, err := srpc.DialHTTP("tcp", hypervisor, 0)
	if err != nil {
		return fmt.Errorf("error connecting to: %s: %s", hypervisor, err)
	}
	defer client.Close()
	initialUpdate := true
	updateHandler := func(update proto.Update) error {
		g.updateVMs(update.VMs, initialUpdate)
		initialUpdate = false
		if waitGroup != nil {
			waitGroup.Done()
			waitGroup = nil
		}
		select {
		case g.eventChannel <- struct{}{}:
		default:
		}
		return nil
	}
	return hyperclient.GetUpdates(client, hyperclient.GetUpdatesParams{
		Logger:        g.logger,
		UpdateHandler: updateHandler,
	})
}

func (g *hypervisorGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdbType, error) {
	startTime := time.Now()
	newMdb := g.generate()
	g.logger.Debugf(1, "Hypervisor generate took: %s\n",
		format.Duration(time.Since(startTime)))
	return newMdb, nil
}

func (g *hypervisorGeneratorType) generate() *mdbType {
	var newMdb mdbType
	g.mutex.Lock()
	defer g.mutex.Unlock()
	for ipAddr, vm := range g.vms {
		if vm.State == proto.StateRunning {
			tags := vm.Tags
			if tags == nil {
				tags = emptyTags
			}
			_, disableUpdates := tags["DisableUpdates"]
			var ownerGroup string
			if len(vm.OwnerGroups) > 0 {
				ownerGroup = vm.OwnerGroups[0]
			}
			newMdb.Machines = append(newMdb.Machines, &mdb.Machine{
				IpAddress:      ipAddr,
				RequiredImage:  tags["RequiredImage"],
				PlannedImage:   tags["PlannedImage"],
				DisableUpdates: disableUpdates,
				OwnerGroup:     ownerGroup,
				OwnerGroups:    vm.OwnerGroups,
				OwnerUsers:     vm.OwnerUsers,
				Tags:           vm.Tags,
			})
		}
	}
	return &newMdb
}

func (g *hypervisorGeneratorType) updateVMs(vms map[string]*proto.VmInfo,
	initialUpdate bool) {
	vmsToDelete := make(map[string]struct{}, len(g.vms))
	if initialUpdate {
		for ipAddr := range g.vms {
			vmsToDelete[ipAddr] = struct{}{}
		}
	}
	startTime := time.Now()
	g.updateVMsWithMap(vms, vmsToDelete)
	g.logger.Debugf(1, "Hypervisor update took: %s\n",
		format.Duration(time.Since(startTime)))
}

func (g *hypervisorGeneratorType) updateVMsWithMap(vms map[string]*proto.VmInfo,
	vmsToDelete map[string]struct{}) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	for ipAddr, vm := range vms {
		if vm == nil || len(vm.Volumes) < 1 {
			delete(g.vms, ipAddr)
		} else {
			g.vms[ipAddr] = vm
			delete(vmsToDelete, ipAddr)
		}
	}
	for ipAddr := range vmsToDelete {
		delete(g.vms, ipAddr)
	}
}
