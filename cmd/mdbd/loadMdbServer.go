package main

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/mdb/mdbd"
)

type mdbServerGeneratorType struct {
	eventChannel chan<- struct{}
	mutex        sync.Mutex
	mdb          *mdb.Mdb
}

func newMdbServerGenerator(params makeGeneratorParams) (generator, error) {
	hostname := params.args[0]
	portNum := uint64(constants.SimpleMdbServerPortNumber)
	if host, port, err := net.SplitHostPort(hostname); err == nil {
		hostname = host
		if portNum, err = strconv.ParseUint(port, 10, 16); err != nil {
			return nil, fmt.Errorf("bad port number: %s: %s", port, err)
		}
	}
	mdbChannel := mdbd.StartMdbDaemon2(
		mdbd.Config{
			Locations:         params.args[1:],
			MdbServerHostname: hostname,
			MdbServerPortNum:  uint(portNum),
		},
		mdbd.Params{Logger: params.logger})
	g := &mdbServerGeneratorType{eventChannel: params.eventChannel}
	params.waitGroup.Add(1)
	go g.daemon(mdbChannel, params.waitGroup)
	return g, nil
}

func (g *mdbServerGeneratorType) daemon(mdbChannel <-chan *mdb.Mdb,
	waitGroup *sync.WaitGroup) {
	for newMdb := range mdbChannel {
		g.mutex.Lock()
		g.mdb = newMdb
		g.mutex.Unlock()
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

func (g *mdbServerGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdbType, error) {
	var newMdb mdbType
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.mdb == nil {
		return &newMdb, nil
	}
	newMdb.Machines = make([]*mdb.Machine, 0, len(g.mdb.Machines))
	for _, machine := range g.mdb.Machines {
		newMdb.Machines = append(newMdb.Machines, &machine)
	}
	return &newMdb, nil
}
