package rrdialer

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
)

var nextPortNumber = 12340

type counterType struct {
	listening chan struct{} // Closed once the listener is up.
	mutex     sync.Mutex    // Protect everything below.
	counter   uint
}

func (c *counterType) get() uint {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.counter
}

func (c *counterType) increment() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.counter++
}

// makeListener listens immediately if start is nil, else once start is closed.
func (e *endpointType) makeListener(start <-chan struct{},
	logger log.Logger) *counterType {
	acceptCounter := &counterType{listening: make(chan struct{})}
	e.address = "localhost:" + strconv.Itoa(nextPortNumber)
	nextPortNumber++
	listen := func() net.Listener {
		listener, err := net.Listen("tcp", e.address)
		if err != nil {
			panic(err)
		}
		close(acceptCounter.listening)
		return listener
	}
	var listener net.Listener
	if start == nil {
		listener = listen()
	}
	go func() {
		if start != nil {
			<-start
			listener = listen()
		}
		for {
			if conn, err := listener.Accept(); err != nil {
				logger.Println(err)
			} else {
				acceptCounter.increment()
				conn.Close()
			}
		}
	}()
	return acceptCounter
}

func waitFor(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDialNoConnections(t *testing.T) {
	dialer := &Dialer{
		logger:    testlogger.New(t),
		rawDialer: &net.Dialer{Timeout: time.Second},
	}
	endpoint50 := &endpointType{
		MeanLatency: 50e-3,
	}
	endpoint100 := &endpointType{
		MeanLatency: 100e-3,
	}
	endpoints := []*endpointType{endpoint50, endpoint100}
	startTime := time.Now()
	_, err := dialer.dialEndpoints(context.Background(), "tcp",
		"localhost:1", endpoints, -1)
	if err == nil {
		t.Fatal("Dial with no working endpoints did not fail")
	}
	if time.Since(startTime) > time.Millisecond*40 {
		t.Fatal("Dial took too long to fail")
	}
}

func TestDialOneIsFastEnough(t *testing.T) {
	dialer := &Dialer{
		logger:    testlogger.New(t),
		rawDialer: &net.Dialer{Timeout: time.Second},
	}
	endpoint50 := &endpointType{
		MeanLatency: 50e-3,
	}
	endpoint100 := &endpointType{
		MeanLatency: 100e-3,
	}
	counter50 := endpoint50.makeListener(nil, dialer.logger)
	start100 := make(chan struct{})
	counter100 := endpoint100.makeListener(start100, dialer.logger)
	endpoints := []*endpointType{endpoint50, endpoint100}
	_, err := dialer.dialEndpoints(context.Background(), "tcp",
		"localhost:1", endpoints, -1)
	if err != nil {
		t.Fatal(err)
	}
	// endpoint100 comes online only after all dial attempts have finished.
	dialer.waitForBackgroundResults(time.Second)
	close(start100)
	<-counter100.listening
	waitFor(t, func() bool { return counter50.get() == 1 },
		"endpoint50 did not connect")
	if counter100.get() != 0 {
		t.Fatal("endpoint100 connected")
	}
}

func TestDialTwoAreFastEnough(t *testing.T) {
	dialer := &Dialer{
		logger:    testlogger.New(t),
		rawDialer: &net.Dialer{Timeout: time.Second},
	}
	endpoint50 := &endpointType{
		MeanLatency: 50e-3,
	}
	endpoint100 := &endpointType{
		MeanLatency: 100e-3,
	}
	endpoint150 := &endpointType{
		LastUpdate:  time.Now(),
		MeanLatency: 150e-3,
	}
	counter50 := endpoint50.makeListener(nil, dialer.logger)
	start100 := make(chan struct{})
	counter100 := endpoint100.makeListener(start100, dialer.logger)
	counter150 := endpoint150.makeListener(nil, dialer.logger)
	endpoints := []*endpointType{endpoint50, endpoint100, endpoint150}
	_, err := dialer.dialEndpoints(context.Background(), "tcp",
		"localhost:1", endpoints, -1)
	if err != nil {
		t.Fatal(err)
	}
	// endpoint100 comes online only after all dial attempts have finished.
	dialer.waitForBackgroundResults(time.Second)
	close(start100)
	<-counter100.listening
	waitFor(t, func() bool {
		return counter50.get() == 1 || counter150.get() == 1
	}, "endpoint50 and endpoint150 did not connect")
	if counter100.get() != 0 {
		t.Fatal("endpoint100 connected")
	}
}
