package main

import (
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

type rpcType struct {
	currentMdb   *mdbType
	eventChannel chan<- struct{}
	logger       log.Logger
	pauseTable   *pauseTableType
	*serverutil.PerUserMethodLimiter
	rwMutex sync.RWMutex
	// Protected by lock.
	updateChannels map[*srpc.Conn]chan<- mdbserver.MdbUpdate
}

type locationFilterType struct {
	locations []string
	sent      map[string]struct{} // Key: hostname sent to the client.
}

func startRpcd(eventChannel chan<- struct{}, pauseTable *pauseTableType,
	logger log.Logger) *rpcType {
	rpcObj := &rpcType{
		eventChannel: eventChannel,
		logger:       logger,
		pauseTable:   pauseTable,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"GetFilteredMdbUpdates": 1,
				"GetMachine":            1,
				"GetMdb":                1,
				"GetMdbUpdates":         1,
				"ListImages":            1,
				"PauseUpdates":          1,
				"ResumeUpdates":         1,
			}),

		updateChannels: make(map[*srpc.Conn]chan<- mdbserver.MdbUpdate),
	}
	srpc.RegisterNameWithOptions("MdbServer", rpcObj, srpc.ReceiverOptions{
		PublicMethods: []string{
			"GetFilteredMdbUpdates",
			"GetMachine",
			"GetMdb",
			"GetMdbUpdates",
			"ListImages",
			"PauseUpdates",
			"ResumeUpdates",
		}})
	return rpcObj
}

func (t *rpcType) GetMachine(conn *srpc.Conn,
	request mdbserver.GetMachineRequest,
	reply *mdbserver.GetMachineResponse) error {
	currentMdb := t.currentMdb
	if currentMdb == nil {
		reply.Error = "no MDB data"
	} else if machine, ok := currentMdb.table[request.Hostname]; !ok {
		reply.Error = request.Hostname + " not in MDB"
	} else {
		reply.Machine = *machine
	}
	return nil
}

func (t *rpcType) GetMdb(conn *srpc.Conn, request mdbserver.GetMdbRequest,
	reply *mdbserver.GetMdbResponse) error {
	currentMdb := t.currentMdb
	if currentMdb == nil {
		return nil
	}
	machines := make([]mdb.Machine, 0, len(currentMdb.Machines))
	for _, machine := range currentMdb.Machines {
		machines = append(machines, *machine)
	}
	reply.Machines = machines
	return nil
}

func (t *rpcType) GetFilteredMdbUpdates(conn *srpc.Conn) error {
	var request mdbserver.GetFilteredMdbUpdatesRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	return t.getMdbUpdates(conn, newLocationFilter(request.Locations))
}

func (t *rpcType) GetMdbUpdates(conn *srpc.Conn) error {
	return t.getMdbUpdates(conn, nil)
}

func (t *rpcType) getMdbUpdates(conn *srpc.Conn,
	filter *locationFilterType) error {
	updateChannel := make(chan mdbserver.MdbUpdate, 10)
	t.rwMutex.Lock()
	t.updateChannels[conn] = updateChannel
	t.rwMutex.Unlock()
	defer func() {
		close(updateChannel)
		t.rwMutex.Lock()
		delete(t.updateChannels, conn)
		t.rwMutex.Unlock()
	}()
	currentMdb := t.currentMdb
	if currentMdb != nil {
		mdbUpdate := mdbserver.MdbUpdate{
			MachinesToAdd: make([]mdb.Machine, 0, len(currentMdb.Machines)),
		}
		for _, machine := range currentMdb.Machines {
			mdbUpdate.MachinesToAdd = append(mdbUpdate.MachinesToAdd, *machine)
		}
		// Sent even if filtered to empty, as it replaces the client's data.
		mdbUpdate = filter.filterUpdate(mdbUpdate)
		if err := conn.Encode(mdbUpdate); err != nil {
			return err
		}
		if err := conn.Flush(); err != nil {
			return err
		}
	}
	closeChannel := conn.GetCloseNotifier()
	for {
		var err error
		select {
		case mdbUpdate := <-updateChannel:
			if isEmptyUpdate(mdbUpdate) {
				t.logger.Printf("Queue for: %s is filling up: dropping client")
				return errors.New("update queue too full")
			}
			mdbUpdate = filter.filterUpdate(mdbUpdate)
			if isEmptyUpdate(mdbUpdate) {
				continue
			}
			if err = conn.Encode(mdbUpdate); err != nil {
				return err
			}
			if err = conn.Flush(); err != nil {
				return err
			}
		case <-closeChannel:
			return nil
		}
		if err != nil {
			if err != io.EOF {
				t.logger.Println(err)
				return err
			} else {
				return nil
			}
		}
	}
}

func (t *rpcType) ListImages(conn *srpc.Conn,
	request mdbserver.ListImagesRequest,
	reply *mdbserver.ListImagesResponse) error {
	currentMdb := t.currentMdb
	if currentMdb == nil {
		return nil
	}
	plannedImages := make(map[string]struct{})
	requiredImages := make(map[string]struct{})
	for _, machine := range currentMdb.Machines {
		plannedImages[machine.PlannedImage] = struct{}{}
		requiredImages[machine.RequiredImage] = struct{}{}
	}
	delete(plannedImages, "")
	delete(requiredImages, "")
	response := mdbserver.ListImagesResponse{
		PlannedImages:  stringutil.ConvertMapKeysToList(plannedImages, false),
		RequiredImages: stringutil.ConvertMapKeysToList(requiredImages, false),
	}
	*reply = response
	return nil
}

func (t *rpcType) PauseUpdates(conn *srpc.Conn,
	request mdbserver.PauseUpdatesRequest,
	reply *mdbserver.PauseUpdatesResponse) error {
	reply.Error = t.pauseUpdates(conn, request, reply)
	return nil
}

func (t *rpcType) pushUpdateToAll(old, new *mdbType) {
	t.currentMdb = new
	updateChannels := t.getUpdateChannels()
	if len(updateChannels) < 1 {
		return
	}
	mdbUpdate := mdbserver.MdbUpdate{}
	if old == nil {
		old = &mdbType{}
	}
	oldMachines := make(map[string]*mdb.Machine, len(old.Machines))
	for _, machine := range old.Machines {
		oldMachines[machine.Hostname] = machine
	}
	for _, newMachine := range new.Machines {
		if oldMachine, ok := oldMachines[newMachine.Hostname]; ok {
			if !newMachine.Compare(*oldMachine) {
				mdbUpdate.MachinesToUpdate = append(mdbUpdate.MachinesToUpdate,
					*newMachine)
			}
		} else {
			mdbUpdate.MachinesToAdd = append(mdbUpdate.MachinesToAdd,
				*newMachine)
		}
	}
	for _, machine := range new.Machines {
		delete(oldMachines, machine.Hostname)
	}
	for name := range oldMachines {
		mdbUpdate.MachinesToDelete = append(mdbUpdate.MachinesToDelete, name)
	}
	if isEmptyUpdate(mdbUpdate) {
		t.logger.Println("Ignoring empty update")
		return
	}
	for _, channel := range updateChannels {
		sendUpdate(channel, mdbUpdate)
	}
}

func (t *rpcType) getUpdateChannels() []chan<- mdbserver.MdbUpdate {
	t.rwMutex.RLock()
	defer t.rwMutex.RUnlock()
	channels := make([]chan<- mdbserver.MdbUpdate, 0, len(t.updateChannels))
	for _, channel := range t.updateChannels {
		channels = append(channels, channel)
	}
	return channels
}

func (t *rpcType) ResumeUpdates(conn *srpc.Conn,
	request mdbserver.ResumeUpdatesRequest,
	reply *mdbserver.ResumeUpdatesResponse) error {
	reply.Error = t.resumeUpdates(conn, request, reply)
	return nil
}

func isEmptyUpdate(mdbUpdate mdbserver.MdbUpdate) bool {
	if len(mdbUpdate.MachinesToAdd) > 0 {
		return false
	}
	if len(mdbUpdate.MachinesToUpdate) > 0 {
		return false
	}
	if len(mdbUpdate.MachinesToDelete) > 0 {
		return false
	}
	return true
}

func newLocationFilter(locations []string) *locationFilterType {
	if len(locations) < 1 {
		return nil
	}
	return &locationFilterType{
		locations: locations,
		sent:      make(map[string]struct{}),
	}
}

func (f *locationFilterType) filterUpdate(
	mdbUpdate mdbserver.MdbUpdate) mdbserver.MdbUpdate {
	if f == nil {
		return mdbUpdate
	}
	var filtered mdbserver.MdbUpdate
	for _, machine := range mdbUpdate.MachinesToAdd {
		if f.match(machine.Location) {
			filtered.MachinesToAdd = append(filtered.MachinesToAdd, machine)
			f.sent[machine.Hostname] = struct{}{}
		}
	}
	for _, machine := range mdbUpdate.MachinesToUpdate {
		_, sent := f.sent[machine.Hostname]
		if f.match(machine.Location) {
			if sent {
				filtered.MachinesToUpdate = append(filtered.MachinesToUpdate,
					machine)
			} else {
				filtered.MachinesToAdd = append(filtered.MachinesToAdd, machine)
				f.sent[machine.Hostname] = struct{}{}
			}
		} else if sent {
			filtered.MachinesToDelete = append(filtered.MachinesToDelete,
				machine.Hostname)
			delete(f.sent, machine.Hostname)
		}
	}
	for _, hostname := range mdbUpdate.MachinesToDelete {
		if _, sent := f.sent[hostname]; sent {
			filtered.MachinesToDelete = append(filtered.MachinesToDelete,
				hostname)
			delete(f.sent, hostname)
		}
	}
	return filtered
}

func (f *locationFilterType) match(location string) bool {
	for _, enclosingLocation := range f.locations {
		if location == enclosingLocation ||
			strings.HasPrefix(location, enclosingLocation+"/") {
			return true
		}
	}
	return false
}

func sendUpdate(channel chan<- mdbserver.MdbUpdate,
	mdbUpdate mdbserver.MdbUpdate) {
	defer func() { recover() }()
	if cap(channel)-len(channel) < 2 {
		// Not enough room for an update and a possible "too much" message next
		// time around: send a "too much" message now.
		channel <- mdbserver.MdbUpdate{}
		return
	}
	channel <- mdbUpdate
}
