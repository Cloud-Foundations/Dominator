package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
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
		return err
	}
	defer client.Close()
	conn, err := client.Call("Hypervisor.GetUpdates")
	if err != nil {
		return err
	}
	defer conn.Close()
	initialUpdate := true
	for {
		var update proto.Update
		if err := conn.Decode(&update); err != nil {
			return err
		}
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
	}
}

func (g *hypervisorGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdb.Mdb, error) {
	var newMdb mdb.Mdb
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
			newMdb.Machines = append(newMdb.Machines, mdb.Machine{
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
	return &newMdb, nil
}

func (g *hypervisorGeneratorType) updateVMs(vms map[string]*proto.VmInfo,
	initialUpdate bool) {
	vmsToDelete := make(map[string]struct{}, len(g.vms))
	if initialUpdate {
		for ipAddr := range g.vms {
			vmsToDelete[ipAddr] = struct{}{}
		}
	}
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
