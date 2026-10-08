package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

type mdbServerGeneratorType struct {
	eventChannel chan<- struct{}
	locations    []string
	logger       log.DebugLogger
	mdbServer    string
	mutex        sync.Mutex
	machines     map[string]mdb.Machine // Key: hostname.
}

func newMdbServerGenerator(params makeGeneratorParams) (generator, error) {
	mdbServer := params.args[0]
	if !strings.Contains(mdbServer, ":") {
		mdbServer = fmt.Sprintf("%s:%d",
			mdbServer, constants.SimpleMdbServerPortNumber)
	}
	g := &mdbServerGeneratorType{
		eventChannel: params.eventChannel,
		locations:    params.args[1:],
		logger:       params.logger,
		mdbServer:    mdbServer,
		machines:     make(map[string]mdb.Machine),
	}
	params.waitGroup.Add(1)
	go g.daemon(params.waitGroup)
	return g, nil
}

func (g *mdbServerGeneratorType) daemon(waitGroup *sync.WaitGroup) {
	for {
		if err := g.getUpdates(waitGroup); err != nil {
			g.logger.Println(err)
			time.Sleep(time.Second)
		}
		waitGroup = nil
	}
}

func (g *mdbServerGeneratorType) getUpdates(waitGroup *sync.WaitGroup) error {
	client, err := srpc.DialHTTP("tcp", g.mdbServer, 0)
	if err != nil {
		return fmt.Errorf("error connecting to: %s: %s", g.mdbServer, err)
	}
	defer client.Close()
	method := "MdbServer.GetMdbUpdates"
	if len(g.locations) > 0 {
		method = "MdbServer.GetFilteredMdbUpdates"
	}
	conn, err := client.Call(method)
	if err != nil {
		return fmt.Errorf("error calling to: %s: %s", g.mdbServer, err)
	}
	defer conn.Close()
	if len(g.locations) > 0 {
		request := mdbserver.GetFilteredMdbUpdatesRequest{
			Locations: g.locations,
		}
		if err := conn.Encode(request); err != nil {
			return fmt.Errorf("error encoding to: %s: %s",
				conn.RemoteAddr(), err)
		}
		if err := conn.Flush(); err != nil {
			return fmt.Errorf("error flushing to: %s: %s",
				conn.RemoteAddr(), err)
		}
	}
	initialUpdate := true
	for {
		var update mdbserver.MdbUpdate
		if err := conn.Decode(&update); err != nil {
			return fmt.Errorf("error decoding from: %s: %s",
				conn.RemoteAddr(), err)
		}
		g.update(update, initialUpdate)
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

func (g *mdbServerGeneratorType) Generate(unused_datacentre string,
	logger log.DebugLogger) (*mdbType, error) {
	startTime := time.Now()
	newMdb := g.generate()
	g.logger.Debugf(1, "MdbServer(%s) generate took: %s\n",
		g.mdbServer, format.Duration(time.Since(startTime)))
	return newMdb, nil
}

func (g *mdbServerGeneratorType) generate() *mdbType {
	var newMdb mdbType
	g.mutex.Lock()
	defer g.mutex.Unlock()
	newMdb.Machines = make([]*mdb.Machine, 0, len(g.machines))
	for _, machine := range g.machines {
		newMdb.Machines = append(newMdb.Machines, &machine)
	}
	return &newMdb
}

func (g *mdbServerGeneratorType) update(update mdbserver.MdbUpdate,
	initialUpdate bool) {
	startTime := time.Now()
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if initialUpdate {
		g.machines = make(map[string]mdb.Machine, len(update.MachinesToAdd))
	}
	for _, machine := range update.MachinesToAdd {
		g.machines[machine.Hostname] = machine
	}
	for _, machine := range update.MachinesToUpdate {
		g.machines[machine.Hostname] = machine
	}
	for _, hostname := range update.MachinesToDelete {
		delete(g.machines, hostname)
	}
	g.logger.Debugf(1, "MdbServer(%s) update took: %s\n",
		g.mdbServer, format.Duration(time.Since(startTime)))
}
