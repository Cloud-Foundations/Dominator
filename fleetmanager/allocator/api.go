package allocator

import (
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/queue"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

type allocationEntryType struct {
	allocation *proto.Allocation
	requestId  proto.RequestId
	username   types.Username
}

type allocateRequestType struct {
	request  proto.AllocateRequest
	response chan<- proto.AllocateResponse
	username types.Username
}

type cancelAllocationType struct {
	authInfo  *srpc.AuthInformation
	requestId proto.RequestId
	response  chan<- error
}

type getRequestType struct {
	requestId proto.RequestId
	response  chan<- *requestInfo
}

type requestInfo struct {
	Allocation *proto.Allocation        `json:",omitempty"`
	Request    *proto.AllocateRequest   `json:",omitempty"`
	Deleted    *proto.DeletedAllocation `json:",omitempty"`
	Username   types.Username
}

type Manager struct {
	active                  bool
	allocateRequestChannel  chan<- allocateRequestType
	cancelAllocationChannel chan<- cancelAllocationType
	closeNotifierChannel    chan<- <-chan proto.AllocationUpdate
	getRequestChannel       chan<- getRequestType
	lastLostHeartbeatTime   time.Time
	listAllocationsChannel  chan<- chan<- []allocationEntryType
	listQueueChannel        chan<- chan<- []proto.AllocateRequestEntry
	lostHeartbeat           bool
	ready                   chan struct{}
	topologyChannel         chan<- *topology.Topology
	updateQueue             queue.BroadcastQueue[proto.AllocationUpdateEntry]
}

type Options struct {
	CreateDeadline   time.Duration
	MaximumQueueSize uint64
}

type Params struct {
	Logger             log.DebugLogger
	Storer             Storer
	UpdateChannelMaker UpdateChannelMaker
	heartbeatTimeout   time.Duration
	managerInterval    time.Duration
	skipDashboard      bool
}

type Storer interface {
	DeleteUpdate(position uint64) error
	DeleteUserRequest(username types.Username, reqId proto.RequestId) error
	ReadUpdates() (uint64, []proto.AllocationUpdateEntry, error)
	ReadUsersQueue() ([]types.Username, error)
	ReadUserQueue(username types.Username) ([]proto.RequestId, error)
	ReadUserRequest(username types.Username, requestId proto.RequestId) (
		proto.AllocateRequest, error)
	WriteUpdate(update proto.AllocationUpdateEntry, position uint64) error
	WriteUsersQueue(usernames []types.Username) error
	WriteUserQueue(username types.Username, requestIDs []proto.RequestId) error
	WriteUserRequest(username types.Username, reqId proto.RequestId,
		req proto.AllocateRequest) error
}

type UpdateChannelMaker interface {
	MakeUpdateChannel(proto.GetUpdatesRequest) <-chan proto.Update
	CloseUpdateChannel(<-chan proto.Update)
}

func New(options Options, params Params) (*Manager, error) {
	return newManager(options, params)
}

// Allocate issues an allocation request. It blocks until the allocator is
// ready.
func (m *Manager) Allocate(authInfo *srpc.AuthInformation,
	request proto.AllocateRequest) (proto.AllocateResponse, error) {
	return m.allocate(authInfo, request)
}

// CancelAllocation cancels an allocation request (that has not yet been
// completed/fulfilled). It blocks until the allocator is ready.
func (m *Manager) CancelAllocation(authInfo *srpc.AuthInformation,
	requestId proto.RequestId) error {
	return m.cancelAllocation(authInfo, requestId)
}

//func (m *Manager) CloseUpdateChannel(channel <-chan proto.AllocationUpdate) {
//	m.closeUpdateChannel(channel)
//}

func (m *Manager) GetUpdateQueue() queue.BroadcastQueue[proto.AllocationUpdateEntry] {
	return m.updateQueue
}

//func (m *Manager) ListAllocations() []proto.AllocationEntry {
//	return m.listAllocations()
//}

//func (m *Manager) ListQueue() []proto.AllocateRequestEntry {
//	return m.listQueue()
//}

func (m *Manager) UpdateTopology(t *topology.Topology) {
	m.topologyChannel <- t
}

func (m *Manager) WriteHtml(writer io.Writer) {
	m.writeHtml(writer)
}
