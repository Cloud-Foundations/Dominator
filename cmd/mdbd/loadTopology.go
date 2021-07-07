package main

import (
	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

type topologyGeneratorType struct {
	location    string
	logger      log.DebugLogger
	topologyDir string
}

func newTopologyGenerator(args []string,
	logger log.DebugLogger) (generator, error) {
	g := &topologyGeneratorType{
		logger:      logger,
		topologyDir: args[0],
	}
	if len(args) > 1 {
		g.location = args[1]
	}
	return g, nil
}

func (g *topologyGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdb.Mdb, error) {
	topo, err := topology.LoadWithParams(topology.Params{
		Logger:      logger,
		TopologyDir: g.topologyDir,
	})
	if err != nil {
		return nil, err
	}
	machines, err := topo.ListMachines(g.location)
	if err != nil {
		return nil, err
	}
	var newMdb mdb.Mdb
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
