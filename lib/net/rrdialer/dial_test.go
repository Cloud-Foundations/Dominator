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
	mutex   sync.Mutex // Protect everything below.
	counter uint
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

func (e *endpointType) makeListener(delay time.Duration,
	logger log.Logger) *counterType {
	var acceptCounter counterType
	e.address = "localhost:" + strconv.Itoa(nextPortNumber)
	nextPortNumber++
	go func() {
		time.Sleep(delay)
		if listener, err := net.Listen("tcp", e.address); err != nil {
			panic(err)
		} else {
			for {
				if conn, err := listener.Accept(); err != nil {
					logger.Println(err)
				} else {
					acceptCounter.increment()
					conn.Close()
				}
			}
		}
	}()
	return &acceptCounter
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
	counter50 := endpoint50.makeListener(0, dialer.logger)
	counter100 := endpoint100.makeListener(time.Millisecond*40, dialer.logger)
	endpoints := []*endpointType{endpoint50, endpoint100}
	time.Sleep(time.Millisecond * 20)
	_, err := dialer.dialEndpoints(context.Background(), "tcp",
		"localhost:1", endpoints, -1)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 20)
	if counter50.get() != 1 {
		t.Fatal("endpoint50 did not connect")
	}
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
	counter50 := endpoint50.makeListener(0, dialer.logger)
	counter100 := endpoint100.makeListener(time.Millisecond*40, dialer.logger)
	counter150 := endpoint150.makeListener(0, dialer.logger)
	endpoints := []*endpointType{endpoint50, endpoint100, endpoint150}
	time.Sleep(time.Millisecond * 20)
	_, err := dialer.dialEndpoints(context.Background(), "tcp",
		"localhost:1", endpoints, -1)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 20)
	if counter50.get() != 1 && counter150.get() != 1 {
		t.Fatal("endpoint50 and endpoint150 did not connect")
	}
	if counter100.get() != 0 {
		t.Fatal("endpoint100 connected")
	}
}
