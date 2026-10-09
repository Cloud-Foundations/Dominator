package main

import (
	"sync"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
)

func checkMdb(t *testing.T, gotMdb *mdbType, want ...mdb.Machine) {
	t.Helper()
	got := make(map[string]mdb.Machine, len(gotMdb.Machines))
	for _, machine := range gotMdb.Machines {
		got[machine.Hostname] = *machine
	}
	if len(got) != len(gotMdb.Machines) {
		t.Errorf("duplicate machines in: %v", gotMdb.Machines)
	}
	if len(got) != len(want) {
		t.Errorf("machines: %v, expected %v", gotMdb.Machines, want)
		return
	}
	for _, wantMachine := range want {
		gotMachine, ok := got[wantMachine.Hostname]
		if !ok {
			t.Errorf("machine: %s missing", wantMachine.Hostname)
		} else if !gotMachine.Compare(wantMachine) {
			t.Errorf("machine: %v, expected %v", gotMachine, wantMachine)
		}
	}
}

func waitForEvent(t *testing.T, eventChannel <-chan struct{}) {
	t.Helper()
	select {
	case <-eventChannel:
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for event")
	}
}

func waitForWaitGroup(t *testing.T, waitGroup *sync.WaitGroup) {
	t.Helper()
	waitChannel := make(chan struct{})
	go func() {
		waitGroup.Wait()
		close(waitChannel)
	}()
	select {
	case <-waitChannel:
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for first update")
	}
}

func startTestGenerator(t *testing.T, args ...string) (generator,
	<-chan struct{}) {
	t.Helper()
	eventChannel := make(chan struct{}, 1)
	waitGroup := &sync.WaitGroup{}
	g, err := newMdbServerGenerator(makeGeneratorParams{
		args:         args,
		eventChannel: eventChannel,
		logger:       nulllogger.New(),
		waitGroup:    waitGroup,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForWaitGroup(t, waitGroup)
	waitForEvent(t, eventChannel)
	return g, eventChannel
}

func generateMdb(t *testing.T, g generator) *mdbType {
	t.Helper()
	newMdb, err := g.Generate("", nulllogger.New())
	if err != nil {
		t.Fatal(err)
	}
	return newMdb
}

func TestGeneratorFollowsServer(t *testing.T) {
	rpcObj, address := startTestServer(t)
	mdb1 := makeMdb(makeMachine("host-a", "dc1"),
		makeMachine("host-b", "dc2"), makeMachine("host-c", "dc3/rack1"))
	rpcObj.pushUpdateToAll(nil, mdb1)
	full, fullEvents := startTestGenerator(t, address)
	filtered, filteredEvents := startTestGenerator(t, address, "dc1", "dc3")
	checkMdb(t, generateMdb(t, full), makeMachine("host-a", "dc1"),
		makeMachine("host-b", "dc2"), makeMachine("host-c", "dc3/rack1"))
	checkMdb(t, generateMdb(t, filtered), makeMachine("host-a", "dc1"),
		makeMachine("host-c", "dc3/rack1"))
	// host-a moves out, host-b moves in and host-c is deleted.
	mdb2 := makeMdb(makeMachine("host-a", "dc2"),
		makeMachine("host-b", "dc1/rack1"))
	rpcObj.pushUpdateToAll(mdb1, mdb2)
	waitForEvent(t, fullEvents)
	waitForEvent(t, filteredEvents)
	checkMdb(t, generateMdb(t, full), makeMachine("host-a", "dc2"),
		makeMachine("host-b", "dc1/rack1"))
	checkMdb(t, generateMdb(t, filtered), makeMachine("host-b", "dc1/rack1"))
}

func TestGeneratorGenerateReturnsCopies(t *testing.T) {
	rpcObj, address := startTestServer(t)
	rpcObj.pushUpdateToAll(nil,
		makeMdb(makeMachine("host-a", "dc1"), makeMachine("host-b", "dc1")))
	g, _ := startTestGenerator(t, address)
	for _, machine := range generateMdb(t, g).Machines {
		machine.Location = "modified"
	}
	checkMdb(t, generateMdb(t, g),
		makeMachine("host-a", "dc1"), makeMachine("host-b", "dc1"))
}

func TestGeneratorEmptyFilteredMdb(t *testing.T) {
	rpcObj, address := startTestServer(t)
	mdb1 := makeMdb(makeMachine("host-a", "dc1"))
	rpcObj.pushUpdateToAll(nil, mdb1)
	g, events := startTestGenerator(t, address, "dc2")
	checkMdb(t, generateMdb(t, g))
	mdb2 := makeMdb(makeMachine("host-a", "dc2"))
	rpcObj.pushUpdateToAll(mdb1, mdb2)
	waitForEvent(t, events)
	checkMdb(t, generateMdb(t, g), makeMachine("host-a", "dc2"))
}
