package allocator

import (
	"fmt"
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/list"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

// dequeue will remove a request from the waiting queue.
func (m *manager) dequeue(authInfo *srpc.AuthInformation,
	requestId proto.RequestId) error {
	request := m.waitingRequestsById[requestId]
	if request == nil {
		return fmt.Errorf("dequeue(): unknown request ID: %s", requestId)
	}
	if authInfo != nil &&
		!authInfo.HaveMethodAccess &&
		authInfo.Username != string(request.username) {
		return fmt.Errorf("no access to resource")
	}
	userQueue := m.userQueues[request.username]
	if userQueue == nil {
		return fmt.Errorf("dequeue(): no user for request ID: %s", requestId)
	}
	userEntry := userQueue.Get(requestId)
	if userEntry == nil {
		return fmt.Errorf("dequeue(): no queue entry for request ID: %s",
			requestId)
	}
	err := m.params.Storer.DeleteUserRequest(request.username, requestId)
	if err != nil {
		return err
	}
	userEntry.Remove()
	if err := m.writeUserQueue(request.username, userQueue); err != nil {
		return err
	}
	if userQueue.Length() < 1 {
		delete(m.userQueues, request.username)
		m.usersQueue.Remove(request.username)
		if err := m.writeUsersQueue(); err != nil {
			return err
		}
	}
	delete(m.waitingRequestsById, requestId)
	return nil
}

// enqueue will add a request to the waiting queue.
func (m *manager) enqueue(request proto.AllocateRequest,
	username types.Username) (*proto.AllocateResponse, error) {
	userQueue := m.userQueues[username]
	var rewriteUsersList bool
	if userQueue == nil {
		userQueue = list.NewUnique[proto.RequestId]()
		m.userQueues[username] = userQueue
		m.usersQueue.PushBack(username)
		rewriteUsersList = true
	}
	requestId := m.makeId()
	m.waitingRequestsById[requestId] = &requestType{
		AllocateRequest: &request,
		username:        username,
	}
	userQueue.PushBack(requestId)
	err := m.params.Storer.WriteUserRequest(username, requestId, request)
	if err != nil {
		return nil, err
	}
	if err := m.writeUserQueue(username, userQueue); err != nil {
		return nil, err
	}
	if rewriteUsersList {
		if err := m.writeUsersQueue(); err != nil {
			return nil, err
		}
	}
	return &proto.AllocateResponse{
		RequestId:      requestId,
		UpdatePosition: m.updateQueue.Position(),
	}, nil
}

func (m *manager) listQueue() []proto.AllocateRequestEntry {
	var retval []proto.AllocateRequestEntry
	m.walkQueue(func(request proto.AllocateRequestEntry) bool {
		retval = append(retval, request)
		return true
	})
	return retval
}

func (m *manager) loadQueues(updates []proto.AllocationUpdateEntry) error {
	for _, update := range updates {
		if allocation := update.Available; allocation != nil {
			var vmHypervisors []string
			for _, vmAlloc := range allocation.VMs {
				host, _, err := net.SplitHostPort(vmAlloc.HypervisorAddress)
				if err != nil {
					return err
				}
				vmHypervisors = append(vmHypervisors, host)
			}
			m.allocations[update.RequestId] = &allocationType{
				allocation:    update.Available,
				request:       update.Request,
				vmHypervisors: vmHypervisors,
				username:      update.Username,
			}
		}
		if deleted := update.Deleted; deleted != nil {
			delType := &deletedType{
				deleted:  deleted,
				request:  update.Request,
				username: update.Username,
			}
			allocation := m.allocations[update.RequestId]
			if allocation != nil {
				delType.allocation = allocation.allocation
				delType.request = allocation.request
			}
			m.deleted[update.RequestId] = delType
			delete(m.allocations, update.RequestId)
		}
	}
	usernames, err := m.params.Storer.ReadUsersQueue()
	if err != nil {
		return err
	}
	for _, username := range usernames {
		userQueue := list.NewUnique[proto.RequestId]()
		requestIDs, err := m.params.Storer.ReadUserQueue(username)
		if err != nil {
			return err
		}
		for _, requestId := range requestIDs {
			request, err := m.params.Storer.ReadUserRequest(username, requestId)
			if err != nil {
				return err
			}
			requestEntry := &requestType{
				AllocateRequest: &request,
				username:        username,
			}
			m.waitingRequestsById[requestId] = requestEntry
			userQueue.PushBack(requestId)
		}
		m.userQueues[username] = userQueue
		m.usersQueue.PushBack(username)
	}
	return nil
}

// walkQueue will walk the request queue in order, calling fn for each queue
// entry. If fn returns true the walk will continue, else it will stop.
func (m *manager) walkQueue(fn func(proto.AllocateRequestEntry) bool) {
	nextEntriesForUsers := make(
		map[types.Username]*list.UniqueListEntry[proto.RequestId])
	m.usersQueue.IterateValues(func(username types.Username) bool {
		nextEntriesForUsers[username] = m.userQueues[username].Front()
		return true
	})
	for {
		var added bool
		cont := m.usersQueue.IterateValues(func(username types.Username) bool {
			if nextEntry := nextEntriesForUsers[username]; nextEntry != nil {
				requestId := nextEntry.Value()
				requestEntry := m.waitingRequestsById[requestId]
				request := proto.AllocateRequestEntry{
					Request:   *requestEntry.AllocateRequest,
					RequestId: requestId,
					Username:  username,
				}
				if !fn(request) {
					return false
				}
				added = true
				nextEntriesForUsers[username] = nextEntry.Next()
			}
			return true
		})
		if !cont || !added {
			return
		}
	}
}

func (m *manager) writeUserQueue(username types.Username,
	userQueue *list.UniqueList[proto.RequestId]) error {
	var requestIDs []proto.RequestId
	userQueue.IterateValues(func(requestId proto.RequestId) bool {
		requestIDs = append(requestIDs, requestId)
		return true
	})
	return m.params.Storer.WriteUserQueue(username, requestIDs)
}

func (m *manager) writeUsersQueue() error {
	var usernames []types.Username
	m.usersQueue.IterateValues(func(username types.Username) bool {
		usernames = append(usernames, username)
		return true
	})
	return m.params.Storer.WriteUsersQueue(usernames)
}
