package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

var setupTopology bool

type topologyGeneratorType struct {
	eventChannel chan<- struct{}
	logger       log.DebugLogger
	mutex        sync.Mutex
	topology     *topology.Topology
}

func newTopologyGenerator(params makeGeneratorParams) (generator, error) {
	if setupTopology {
		return nil, errors.New("only one Topology driver permitted")
	}
	var topologyUrl, localRepositoryDir, topologyDir string
	interval := time.Duration(*fetchInterval) * time.Second
	if fi, err := os.Stat(params.args[0]); err == nil && fi.IsDir() {
		localRepositoryDir = params.args[0]
	} else {
		topologyUrl = params.args[0]
		localRepositoryDir = filepath.Join(*stateDir, "topology")
		if strings.HasPrefix(topologyUrl, "git@") {
			if interval < 59*time.Second {
				interval = 59 * time.Second
			}
		}
	}
	if len(params.args) > 1 {
		topologyDir = params.args[1]
	}
	topoChannel, err := topology.Watch(topologyUrl, localRepositoryDir,
		topologyDir, interval, params.logger)
	if err != nil {
		return nil, err
	}
	g := &topologyGeneratorType{
		eventChannel: params.eventChannel,
		logger:       params.logger,
	}
	params.waitGroup.Add(1)
	go g.daemon(topoChannel, params.waitGroup)
	setupTopology = true
	return g, nil
}

func (g *topologyGeneratorType) daemon(topoChannel <-chan *topology.Topology,
	waitGroup *sync.WaitGroup) {
	firstTopo := <-topoChannel
	g.mutex.Lock()
	g.topology = firstTopo
	g.mutex.Unlock()
	waitGroup.Done()
	select {
	case g.eventChannel <- struct{}{}:
	default:
	}
	for topo := range topoChannel {
		g.logger.Println("Received new topology")
		g.mutex.Lock()
		g.topology = topo
		g.mutex.Unlock()
		select {
		case g.eventChannel <- struct{}{}:
		default:
		}
	}
}

func (g *topologyGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdb.Mdb, error) {
	g.mutex.Lock()
	topo := g.topology
	g.mutex.Unlock()
	var newMdb mdb.Mdb
	if topo == nil {
		return &newMdb, nil
	}
	machines, err := topo.ListMachines("")
	if err != nil {
		return nil, err
	}
	for _, machine := range machines {
		var ipAddr string
		if len(machine.HostIpAddress) > 0 {
			ipAddr = machine.HostIpAddress.String()
		}
		tags := machine.Tags
		if tags == nil {
			tags = emptyTags
		}
		_, disableUpdates := tags["DisableUpdates"]
		newMdb.Machines = append(newMdb.Machines, mdb.Machine{
			Hostname:       machine.Hostname,
			IpAddress:      ipAddr,
			RequiredImage:  tags["RequiredImage"],
			PlannedImage:   tags["PlannedImage"],
			DisableUpdates: disableUpdates,
			Tags:           machine.Tags,
		})
	}
	return &newMdb, nil
}

func (g *topologyGeneratorType) GetVariables() (map[string]string, error) {
	g.mutex.Lock()
	topo := g.topology
	g.mutex.Unlock()
	return topo.Variables, nil
}
