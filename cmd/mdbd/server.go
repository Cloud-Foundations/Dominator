package main

import (
	"errors"
	"io"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/serverutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

type rpcType struct {
	currentMdb *mdb.Mdb
	logger     log.Logger
	*serverutil.PerUserMethodLimiter
	rwMutex sync.RWMutex
	// Protected by lock.
	updateChannels map[*srpc.Conn]chan<- mdbserver.MdbUpdate
}

func startRpcd(logger log.Logger) *rpcType {
	rpcObj := &rpcType{
		logger: logger,
		PerUserMethodLimiter: serverutil.NewPerUserMethodLimiter(
			map[string]uint{
				"ListImages": 1,
			}),

		updateChannels: make(map[*srpc.Conn]chan<- mdbserver.MdbUpdate),
	}
	srpc.RegisterNameWithOptions("MdbServer", rpcObj, srpc.ReceiverOptions{
		PublicMethods: []string{
			"ListImages",
		}})
	return rpcObj
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

func (t *rpcType) GetMdbUpdates(conn *srpc.Conn) error {
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
		mdbUpdate := mdbserver.MdbUpdate{MachinesToAdd: currentMdb.Machines}
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
			if err = conn.Encode(mdbUpdate); err != nil {
				break
			}
			if err = conn.Flush(); err != nil {
				break
			}
		case <-closeChannel:
			break
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

func (t *rpcType) pushUpdateToAll(old, new *mdb.Mdb) {
	t.currentMdb = new
	updateChannels := t.getUpdateChannels()
	if len(updateChannels) < 1 {
		return
	}
	mdbUpdate := mdbserver.MdbUpdate{}
	if old == nil {
		old = &mdb.Mdb{}
	}
	oldMachines := make(map[string]mdb.Machine, len(old.Machines))
	for _, machine := range old.Machines {
		oldMachines[machine.Hostname] = machine
	}
	for _, newMachine := range new.Machines {
		if oldMachine, ok := oldMachines[newMachine.Hostname]; ok {
			if !newMachine.Compare(oldMachine) {
				mdbUpdate.MachinesToUpdate = append(mdbUpdate.MachinesToUpdate,
					newMachine)
			}
		} else {
			mdbUpdate.MachinesToAdd = append(mdbUpdate.MachinesToAdd,
				newMachine)
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
