package main

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

var (
	machine0 = mdb.Machine{Hostname: "testhost-0"}
	machine1 = mdb.Machine{Hostname: "testhost-1"}
	machine2 = mdb.Machine{
		Hostname: "testhost-2",
		Tags: tags.Tags{
			"DisruptionManagerReadyTimeout": "1s",
			"DisruptionManagerReadyUrl":     "http://{{.Hostname}}:1234/readiness",
		},
	}
	machine3 = mdb.Machine{
		Hostname: "testhost-3",
		Tags: tags.Tags{
			"DisruptionManagerReadyTimeout": "100ms",
			"DisruptionManagerReadyUrl":     "TO BE MUTATED",
		},
	}
	machine4 = mdb.Machine{
		Hostname: "testhost-4",
		Tags: tags.Tags{
			"DisruptionManagerReadyTimeout": "100ms",
			"DisruptionManagerReadyUrl":     "TO BE MUTATED",
		},
	}

	ok = []byte("ok")
)

func okHandler(w http.ResponseWriter, req *http.Request) {
	w.Write(ok)
}

func TestBasicSequence(t *testing.T) {
	logger := testlogger.New(t)
	dm, err := newDisruptionManager("", time.Second, logger)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := dm.check(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateDenied {
		t.Fatalf("initial state: %s != %s", state, proto.DisruptionStateDenied)
	}
	state, _, err = dm.request(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStatePermitted {
		t.Fatalf("after request state: %s != %s",
			state, proto.DisruptionStatePermitted)
	}
	state, _, err = dm.request(machine1)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateRequested {
		t.Fatalf("after request state: %s != %s",
			state, proto.DisruptionStateRequested)
	}
	state, _, err = dm.cancel(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateDenied {
		t.Fatalf("after cancel state: %s != %s",
			state, proto.DisruptionStateDenied)
	}
	state, _, err = dm.check(machine1)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStatePermitted {
		t.Fatalf("after check state: %s != %s",
			state, proto.DisruptionStatePermitted)
	}
	state, _, err = dm.cancel(machine1)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateDenied {
		t.Fatalf("after cancel state: %s != %s",
			state, proto.DisruptionStateDenied)
	}
}

func TestMakeWaitData(t *testing.T) {
	logger := testlogger.New(t)
	waitData := makeWaitData(machine2, logger)
	if waitData == nil {
		t.Fatalf("failed to parse waitData")
	}
	readyTimeout := time.Until(waitData.ReadyTimeout)
	if readyTimeout < 900*time.Millisecond &&
		readyTimeout > 1100*time.Millisecond {
		t.Errorf("ReadyTimeout: %s != 1s", waitData.ReadyTimeout)
	}
	expectedUrl := "http://testhost-2:1234/readiness"
	if waitData.ReadyUrl != expectedUrl {
		t.Errorf("ReadyUrl: %s != %s", waitData.ReadyUrl, expectedUrl)
	}
}

func TestReadiness(t *testing.T) {
	logger := testlogger.New(t)
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		t.Fatal(err)
	}
	serveMux := http.NewServeMux()
	go http.Serve(listener, serveMux)
	time.Sleep(time.Millisecond)
	machine3.Tags["DisruptionManagerReadyUrl"] =
		fmt.Sprintf("http://%s/readiness", listener.Addr())
	dm, err := newDisruptionManager("", time.Second, logger)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := dm.request(machine3)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStatePermitted {
		t.Fatalf("initial state: %s != %s",
			state, proto.DisruptionStatePermitted)
	}
	state, _, err = dm.cancel(machine3)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateDenied {
		t.Fatalf("after cancel state: %s != %s",
			state, proto.DisruptionStateDenied)
	}
	time.Sleep(30 * time.Millisecond)
	state, _, err = dm.request(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateRequested {
		t.Fatalf("after first sleep request state: %s != %s",
			state, proto.DisruptionStateRequested)
	}
	serveMux.HandleFunc("/readiness", okHandler)
	time.Sleep(30 * time.Millisecond)
	state, _, err = dm.check(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStatePermitted {
		t.Fatalf("after second sleep request state: %s != %s",
			state, proto.DisruptionStatePermitted)
	}
}

func TestReadinessTimeout(t *testing.T) {
	logger := testlogger.New(t)
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		t.Fatal(err)
	}
	serveMux := http.NewServeMux()
	go http.Serve(listener, serveMux)
	time.Sleep(time.Millisecond)
	machine4.Tags["DisruptionManagerReadyUrl"] =
		fmt.Sprintf("http://%s/readiness", listener.Addr())
	dm, err := newDisruptionManager("", time.Second, logger)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err := dm.request(machine4)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStatePermitted {
		t.Fatalf("initial state: %s != %s",
			state, proto.DisruptionStatePermitted)
	}
	startTime := time.Now()
	state, _, err = dm.cancel(machine4)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateDenied {
		t.Fatalf("after cancel state: %s != %s",
			state, proto.DisruptionStateDenied)
	}
	state, _, err = dm.request(machine0)
	if err != nil {
		t.Fatal(err)
	}
	if state != proto.DisruptionStateRequested {
		t.Fatalf("after first sleep request state: %s != %s",
			state, proto.DisruptionStateRequested)
	}
	stopTime := startTime.Add(200 * time.Millisecond)
	var permitted bool
	for ; time.Until(stopTime) > 0; time.Sleep(10 * time.Millisecond) {
		state, _, err = dm.check(machine0)
		if err != nil {
			t.Fatal(err)
		}
		if state == proto.DisruptionStatePermitted {
			permitted = true
			break
		}
	}
	timeTaken := time.Since(startTime)
	if !permitted {
		t.Fatalf("never permitted after: %s", format.Duration(timeTaken))
	} else if timeTaken < 100*time.Millisecond {
		t.Fatalf("permitted before timeout, after: %s",
			format.Duration(timeTaken))
	}
}
