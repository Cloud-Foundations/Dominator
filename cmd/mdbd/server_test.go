package main

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

var (
	testServerOnce    sync.Once
	testServerRpcObj  *rpcType
	testServerAddress string
)

func makeMachine(hostname, location string) mdb.Machine {
	return mdb.Machine{Hostname: hostname, Location: location}
}

func makeMdb(machines ...mdb.Machine) *mdbType {
	newMdb := &mdbType{}
	for index := range machines {
		newMdb.Machines = append(newMdb.Machines, &machines[index])
	}
	return newMdb
}

func checkHostnames(t *testing.T, field string, got []string,
	want ...string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s: %v, expected %v", field, got, want)
	}
}

func checkUpdate(t *testing.T, mdbUpdate mdbserver.MdbUpdate,
	add, update, del []string) {
	t.Helper()
	var added, updated []string
	for _, machine := range mdbUpdate.MachinesToAdd {
		added = append(added, machine.Hostname)
	}
	for _, machine := range mdbUpdate.MachinesToUpdate {
		updated = append(updated, machine.Hostname)
	}
	checkHostnames(t, "MachinesToAdd", added, add...)
	checkHostnames(t, "MachinesToUpdate", updated, update...)
	checkHostnames(t, "MachinesToDelete", mdbUpdate.MachinesToDelete, del...)
}

// newSeededFilter returns a filter for dc1 which has sent host-a (in dc1) and
// dropped host-b (in dc2).
func newSeededFilter(t *testing.T) *locationFilterType {
	t.Helper()
	filter := newLocationFilter([]string{"dc1"})
	filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToAdd: []mdb.Machine{
			makeMachine("host-a", "dc1/rack1"),
			makeMachine("host-b", "dc2/rack1"),
		},
	})
	return filter
}

func TestNilFilterPassesThrough(t *testing.T) {
	filter := newLocationFilter(nil)
	if filter != nil {
		t.Fatal("filter for no locations is not nil")
	}
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToAdd:    []mdb.Machine{makeMachine("host-a", "dc1")},
		MachinesToUpdate: []mdb.Machine{makeMachine("host-b", "dc2")},
		MachinesToDelete: []string{"host-c"},
	})
	checkUpdate(t, mdbUpdate, []string{"host-a"}, []string{"host-b"},
		[]string{"host-c"})
}

func TestMatch(t *testing.T) {
	filter := newLocationFilter([]string{"dc1/rack1", "dc2"})
	for location, want := range map[string]bool{
		"dc1/rack1":   true,
		"dc1/rack1/x": true,
		"dc2":         true,
		"dc2/rack9":   true,
		"dc1":         false,
		"dc1/rack10":  false,
		"dc20":        false,
		"":            false,
	} {
		if got := filter.match(location); got != want {
			t.Errorf("match(%q): %v, expected %v", location, got, want)
		}
	}
}

func TestInitialDumpIsFiltered(t *testing.T) {
	filter := newLocationFilter([]string{"dc1"})
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToAdd: []mdb.Machine{
			makeMachine("host-a", "dc1/rack1"),
			makeMachine("host-b", "dc2/rack1"),
		},
	})
	checkUpdate(t, mdbUpdate, []string{"host-a"}, nil, nil)
}

func TestUpdateWithinFilter(t *testing.T) {
	filter := newSeededFilter(t)
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-a", "dc1/rack2")},
	})
	checkUpdate(t, mdbUpdate, nil, []string{"host-a"}, nil)
}

func TestUpdateOutsideFilterIsDropped(t *testing.T) {
	filter := newSeededFilter(t)
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-b", "dc2/rack2")},
	})
	if !isEmptyUpdate(mdbUpdate) {
		t.Errorf("update: %v, expected empty", mdbUpdate)
	}
}

func TestMoveInBecomesAdd(t *testing.T) {
	filter := newSeededFilter(t)
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-b", "dc1/rack1")},
	})
	checkUpdate(t, mdbUpdate, []string{"host-b"}, nil, nil)
	mdbUpdate = filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-b", "dc1/rack2")},
	})
	checkUpdate(t, mdbUpdate, nil, []string{"host-b"}, nil)
}

func TestMoveOutBecomesDelete(t *testing.T) {
	filter := newSeededFilter(t)
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-a", "dc2/rack1")},
	})
	checkUpdate(t, mdbUpdate, nil, nil, []string{"host-a"})
	mdbUpdate = filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToUpdate: []mdb.Machine{makeMachine("host-a", "dc1/rack1")},
	})
	checkUpdate(t, mdbUpdate, []string{"host-a"}, nil, nil)
}

func TestDeleteOnlySentMachines(t *testing.T) {
	filter := newSeededFilter(t)
	mdbUpdate := filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToDelete: []string{"host-a", "host-b", "host-c"},
	})
	checkUpdate(t, mdbUpdate, nil, nil, []string{"host-a"})
	mdbUpdate = filter.filterUpdate(mdbserver.MdbUpdate{
		MachinesToDelete: []string{"host-a"},
	})
	if !isEmptyUpdate(mdbUpdate) {
		t.Errorf("second delete: %v, expected empty", mdbUpdate)
	}
}

func callStream(t *testing.T, address, method string,
	request interface{}) *srpc.Conn {
	t.Helper()
	client, err := srpc.DialHTTP("tcp", address, time.Second*5)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	conn, err := client.Call(method)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	if request != nil {
		if err := conn.Encode(request); err != nil {
			t.Fatal(err)
		}
		if err := conn.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	return conn
}

func receiveUpdate(t *testing.T, conn *srpc.Conn) mdbserver.MdbUpdate {
	t.Helper()
	type result struct {
		mdbUpdate mdbserver.MdbUpdate
		err       error
	}
	resultChannel := make(chan result, 1)
	go func() {
		var mdbUpdate mdbserver.MdbUpdate
		err := conn.Decode(&mdbUpdate)
		resultChannel <- result{mdbUpdate, err}
	}()
	select {
	case r := <-resultChannel:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.mdbUpdate
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for update")
	}
	return mdbserver.MdbUpdate{}
}

// startTestServer starts the MdbServer once, since srpc registration is
// process-wide.
func startTestServer(t *testing.T) (*rpcType, string) {
	t.Helper()
	testServerOnce.Do(func() {
		listener, err := net.Listen("tcp", "localhost:")
		if err != nil {
			t.Fatal(err)
		}
		testServerRpcObj = startRpcd(make(chan struct{}, 1), nil,
			nulllogger.New())
		testServerAddress = listener.Addr().String()
		go func() {
			if err := http.Serve(listener, nil); err != nil {
				panic(err)
			}
		}()
	})
	if testServerRpcObj == nil {
		t.Fatal("test server not started")
	}
	return testServerRpcObj, testServerAddress
}

func TestStreams(t *testing.T) {
	rpcObj, address := startTestServer(t)
	mdb1 := makeMdb(makeMachine("host-a", "dc1"), makeMachine("host-b", "dc2"))
	rpcObj.pushUpdateToAll(nil, mdb1)
	full := callStream(t, address, "MdbServer.GetMdbUpdates", nil)
	filtered := callStream(t, address, "MdbServer.GetFilteredMdbUpdates",
		mdbserver.GetFilteredMdbUpdatesRequest{Locations: []string{"dc1"}})
	checkUpdate(t, receiveUpdate(t, full),
		[]string{"host-a", "host-b"}, nil, nil)
	checkUpdate(t, receiveUpdate(t, filtered), []string{"host-a"}, nil, nil)
	// host-a moves out of dc1 and host-c is added to dc1.
	mdb2 := makeMdb(makeMachine("host-a", "dc2"), makeMachine("host-b", "dc2"),
		makeMachine("host-c", "dc1"))
	rpcObj.pushUpdateToAll(mdb1, mdb2)
	checkUpdate(t, receiveUpdate(t, full),
		[]string{"host-c"}, []string{"host-a"}, nil)
	checkUpdate(t, receiveUpdate(t, filtered),
		[]string{"host-c"}, nil, []string{"host-a"})
	// host-a moves back and host-b, never sent to the filtered stream, goes.
	mdb3 := makeMdb(makeMachine("host-a", "dc1"), makeMachine("host-c", "dc1"))
	rpcObj.pushUpdateToAll(mdb2, mdb3)
	checkUpdate(t, receiveUpdate(t, full),
		nil, []string{"host-a"}, []string{"host-b"})
	checkUpdate(t, receiveUpdate(t, filtered), []string{"host-a"}, nil, nil)
	// Nothing in dc1 changes, so the filtered stream must send nothing.
	mdb4 := makeMdb(makeMachine("host-a", "dc1"), makeMachine("host-c", "dc1"),
		makeMachine("host-d", "dc2"))
	rpcObj.pushUpdateToAll(mdb3, mdb4)
	checkUpdate(t, receiveUpdate(t, full), []string{"host-d"}, nil, nil)
	mdb5 := makeMdb(makeMachine("host-a", "dc1"), makeMachine("host-d", "dc2"))
	rpcObj.pushUpdateToAll(mdb4, mdb5)
	checkUpdate(t, receiveUpdate(t, full), nil, nil, []string{"host-c"})
	checkUpdate(t, receiveUpdate(t, filtered), nil, nil, []string{"host-c"})
}
