package slavedriver

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

type fakeDatabase struct {
}

func (db *fakeDatabase) load() (*slaveRoll, error) {
	return nil, nil
}

func (db *fakeDatabase) save(slaves slaveRoll) error {
	return nil
}

type fakeTrader struct {
	createDelay  time.Duration
	destroyDelay time.Duration
	mutex        sync.RWMutex
	numExisting  int
	sequence     int
}

func fakeDialer(string, string, time.Duration) (*srpc.Client, error) {
	return srpc.NewFakeClient(srpc.FakeClientOptions{}), nil
}

func (trader *fakeTrader) Close() error {
	return nil
}

func (trader *fakeTrader) CreateSlave() (SlaveInfo, error) {
	trader.mutex.Lock()
	defer trader.mutex.Unlock()
	time.Sleep(trader.createDelay)
	slave := SlaveInfo{
		Identifier: fmt.Sprintf("%d", trader.sequence),
	}
	trader.numExisting++
	trader.sequence++
	return slave, nil
}

func (trader *fakeTrader) DestroySlave(identifier string) error {
	trader.mutex.Lock()
	defer trader.mutex.Unlock()
	time.Sleep(trader.destroyDelay)
	trader.numExisting--
	return nil
}

func (trader *fakeTrader) getSequence() int {
	trader.mutex.RLock()
	defer trader.mutex.RUnlock()
	return trader.sequence
}

func TestOneGet(t *testing.T) {
	logger := testlogger.NewWithTimestamps(t)
	trader := &fakeTrader{
		createDelay:  100 * time.Microsecond,
		destroyDelay: 10 * time.Microsecond,
	}
	slaveDriver, err := newSlaveDriver(
		SlaveDriverOptions{
			MaximumIdleSlaves: 2,
			MinimumIdleSlaves: 1,
		},
		trader,
		fakeDialer,
		&fakeDatabase{},
		logger)
	if err != nil {
		logger.Fatal(err)
	}
	startTime := time.Now()
	slave, err := slaveDriver.GetSlave()
	if err != nil {
		logger.Fatal(err)
	}
	logger.Printf("got slave: %s after: %s",
		slave, format.Duration(time.Since(startTime)))
	time.Sleep(time.Millisecond)
	slave.Destroy()
	logger.Print("slave.Destroy() returned")
	time.Sleep(10 * time.Millisecond)
	if sequence := trader.getSequence(); sequence != 2 {
		logger.Fatalf("sequence: %d != 2", sequence)
	} else {
		logger.Printf("sequence: %d, as expected", sequence)
	}
}

func TestTwoGets(t *testing.T) {
	logger := testlogger.NewWithTimestamps(t)
	trader := &fakeTrader{
		createDelay:  100 * time.Millisecond,
		destroyDelay: 10 * time.Millisecond,
	}
	slaveDriver, err := newSlaveDriver(
		SlaveDriverOptions{
			MaximumIdleSlaves: 2,
			MinimumIdleSlaves: 1,
		},
		trader,
		fakeDialer,
		&fakeDatabase{},
		logger)
	if err != nil {
		logger.Fatal(err)
	}
	startTime := time.Now()
	slave, err := slaveDriver.GetSlave()
	if err != nil {
		logger.Fatal(err)
	}
	timeTaken := time.Since(startTime)
	if timeTaken > 150*time.Millisecond {
		logger.Fatalf("got slave: %s after: %s",
			slave, format.Duration(timeTaken))
	}
	logger.Printf("got slave: %s after: %s", slave, format.Duration(timeTaken))
	slave.Destroy()
	logger.Print("destroyed slave")
	startTime = time.Now()
	slave, err = slaveDriver.GetSlave()
	if err != nil {
		logger.Fatal(err)
	}
	timeTaken = time.Since(startTime)
	if timeTaken > 150*time.Millisecond {
		logger.Fatalf("got slave: %s after: %s",
			slave, format.Duration(timeTaken))
	}
	logger.Printf("got slave: %s after: %s", slave, format.Duration(timeTaken))
	slave.Destroy()
	logger.Print("destroyed slave")
}
