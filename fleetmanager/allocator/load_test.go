package allocator

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
	"github.com/Cloud-Foundations/Dominator/lib/queue"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func dumpStack(t *testing.T) {
	buffer := make([]byte, 1<<20)
	nBytes := runtime.Stack(buffer, true)
	t.Output().Write(buffer[:nBytes])
	t.Fatal("aborting test")
}

type fakeStorer struct {
	logger       log.DebugLogger
	positionLock sync.Mutex
	position     uint64
}

func newFakeStorer(logger log.DebugLogger) *fakeStorer {
	return &fakeStorer{
		logger: logger,
	}
}

func (s *fakeStorer) DeleteUpdate(position uint64) error {
	s.logger.Printf("DeleteUpdate(%d)\n", position)
	return nil
}

func (*fakeStorer) DeleteUserRequest(types.Username, proto.RequestId) error {
	return nil
}

func (*fakeStorer) ReadUpdates() (uint64, []proto.AllocationUpdateEntry,
	error) {
	return 0, nil, nil
}

func (*fakeStorer) ReadUsersQueue() ([]types.Username, error) {
	return nil, nil
}

func (*fakeStorer) ReadUserQueue(types.Username) ([]proto.RequestId, error) {
	return nil, nil
}

func (*fakeStorer) ReadUserRequest(types.Username, proto.RequestId) (
	proto.AllocateRequest, error) {
	return proto.AllocateRequest{}, nil
}

func (s *fakeStorer) WriteUpdate(update proto.AllocationUpdateEntry,
	position uint64) error {
	s.positionLock.Lock()
	sPos := s.position
	s.position++
	s.positionLock.Unlock()
	if update.Available != nil {
		s.logger.Printf(
			"WriteUpdate(%s) available: %+v, position: %d,%d, req: %+v\n",
			update.RequestId, *update.Available, position, sPos,
			*update.Request)
	} else if update.Deleted != nil {
		s.logger.Printf(
			"WriteUpdate.Deleted(%s) deleted: %+v, position: %d,%d\n",
			update.RequestId, *update.Deleted, position, sPos)
	} else {
		s.logger.Printf("WriteUpdate(%+v, %d,%d)\n", update, position, sPos)
	}
	if position != sPos {
		return fmt.Errorf("position: queue: %d != storer: %d", position, sPos)
	}
	return nil
}

func (*fakeStorer) WriteUsersQueue([]types.Username) error { return nil }

func (*fakeStorer) WriteUserQueue(types.Username, []proto.RequestId) error {
	return nil
}

func (*fakeStorer) WriteUserRequest(types.Username, proto.RequestId,
	proto.AllocateRequest) error {
	return nil
}

type fakeUpdater struct {
	logger        log.DebugLogger
	updateChannel chan proto.Update
	wg            *sync.WaitGroup
	mutex         sync.Mutex // Protect everything below.
	ipOctet       byte
}

func newFakeUpdater(wg *sync.WaitGroup, logger log.DebugLogger) (
	*fakeUpdater, error) {
	updater := &fakeUpdater{
		logger:        logger,
		updateChannel: make(chan proto.Update), // Need this to be synchronous.
		wg:            wg,
	}
	return updater, nil
}

func (updater *fakeUpdater) CloseUpdateChannel(ch <-chan proto.Update) {
	if ch != updater.updateChannel {
		panic("channel mismatch")
	}
	close(updater.updateChannel)
}

func (updater *fakeUpdater) MakeUpdateChannel(
	proto.GetUpdatesRequest) <-chan proto.Update {
	return updater.updateChannel
}

func (updater *fakeUpdater) createVm(updateEntry proto.AllocationUpdateEntry) {
	updater.mutex.Lock()
	ip := [4]byte{1, 2, 3, updater.ipOctet}
	updater.ipOctet++
	updater.mutex.Unlock()
	ipAddress := net.IP(ip[:])
	ipAddr := ipAddress.String()
	vmSpec := updateEntry.Request.VMs[0]
	vm := &hyper_proto.VmInfo{
		Address:     hyper_proto.Address{IpAddress: ipAddress},
		Hostname:    ipAddr,
		MemoryInMiB: vmSpec.MemoryInMiB,
		MilliCPUs:   vmSpec.MilliCPUs,
		OwnerUsers:  []string{string(updateEntry.Username)},
		SubnetId:    vmSpec.NetworkInterfaces[0].SubnetId,
		Tags: tags.Tags{
			"AllocationRequestId": string(updateEntry.RequestId),
		},
		Volumes: []hyper_proto.Volume{{
			Size: uint64(vmSpec.Volumes[0].Size),
		}},
	}
	updater.updateChannel <- proto.Update{
		ChangedVMs:     map[string]*hyper_proto.VmInfo{ipAddr: vm},
		VmToHypervisor: map[string]string{ipAddr: "hyper0"},
	}
	updater.wg.Done()
}

func (updater *fakeUpdater) initialise() {
	updater.updateChannel <- proto.Update{
		ChangedHypervisors: map[string]proto.HypervisorData{
			"hyper0": proto.HypervisorData{
				AvailableMemory:  1 << 40,
				NumFreeAddresses: map[string]uint{"subnet": 100},
			}},
		ChangedMachines: []*proto.Machine{{
			MachineData: proto.MachineData{
				MemoryInMiB:      1 << 20,
				NumCPUs:          64,
				TotalVolumeBytes: 10 << 40,
			},
			NetworkEntry: proto.NetworkEntry{
				Hostname: "hyper0",
				SubnetId: "subnet",
			},
		}},
	}
	updater.updateChannel <- proto.Update{} // Ensure manager processed it.
}

func (updater *fakeUpdater) processAllocationQueue(
	allocationQueue <-chan queue.BroadcastEntry[proto.AllocationUpdateEntry]) {
	for allocationEntry := range allocationQueue {
		updater.logger.Printf("allocation update entry: %+v\n", allocationEntry)
		if allocationEntry.Value.Available != nil {
			updater.createVm(allocationEntry.Value)
		}
	}
}

func (m *Manager) makeTestAllocationRequest(wg *sync.WaitGroup,
	errChannel chan<- error, multiplier uint) {
	wg.Add(1)
	_, err := m.Allocate(
		&srpc.AuthInformation{Username: "tester"},
		proto.AllocateRequest{
			Deadline: time.Now().Add(5 * time.Second),
			VMs: []proto.VmAllocationSpecification{{
				MemoryInMiB: uint64(multiplier * 1024),
				MilliCPUs:   multiplier * 1000,
				NetworkInterfaces: []proto.NetworkInterfaceSpecification{{
					SubnetId: "subnet",
				}},
				Volumes: []proto.VolumeSpecification{{Size: 1 << 30}},
			}},
		})
	errChannel <- err
}

func (m *Manager) makeTestAllocationRequests(t *testing.T, wg *sync.WaitGroup,
	timer *time.Timer, numRequests uint) {
	t.Logf("queuing %d alllocation requests", numRequests)
	// Make allocation requests in goroutines so that the test fuction is
	// isolated from deadlocks, but collect errors so they are logged here.
	errChannel := make(chan error, numRequests)
	go func() {
		for index := range numRequests {
			m.makeTestAllocationRequest(wg, errChannel, index+1)
		}
	}()
	for range numRequests {
		select {
		case err := <-errChannel:
			if err != nil {
				t.Fatal(err)
			}
		case <-timer.C:
			t.Log("timed out waiting for completion")
			dumpStack(t)
		}
	}
	time.Sleep(time.Millisecond)
}

func TestFullQueue(t *testing.T) {
	logger := testlogger.New(t)
	topo, err := topology.Load("testdata/topology")
	if err != nil {
		t.Fatal(err)
	}
	wg := &sync.WaitGroup{}
	updater, err := newFakeUpdater(wg, logger)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(Options{
		CreateDeadline:   5 * time.Second,
		MaximumQueueSize: 20,
	},
		Params{
			Logger:             logger,
			Storer:             newFakeStorer(logger),
			UpdateChannelMaker: updater,
			heartbeatTimeout:   time.Second,
			managerInterval:    10 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	allocationQueue := m.GetUpdateQueue()
	go updater.processAllocationQueue(allocationQueue.Subscribe(0))
	m.UpdateTopology(topo)
	m.UpdateTopology(nil) // Ensure manager processed it.
	updater.initialise()
	timer := time.NewTimer(2 * time.Second)
	for range 10 {
		m.makeTestAllocationRequests(t, wg, timer, 7)
	}
	completionChannel := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		completionChannel <- struct{}{}
	}()
	select {
	case <-completionChannel:
	case <-timer.C:
		t.Log("timed out waiting for completion, stacktrace follows")
		dumpStack(t)
	}
	// Give time for goroutines to finish logging.
	time.Sleep(10 * time.Millisecond)
}
