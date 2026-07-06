package fsstorer

import (
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

type Storer struct {
	directoriesMade map[string]struct{}
	logger          log.DebugLogger
	topDir          string
}

func New(topDir string, logger log.DebugLogger) (*Storer, error) {
	return newStorer(topDir, logger)
}

func (s *Storer) DeleteUpdate(position uint64) error {
	return s.deleteUpdate(position)
}

func (s *Storer) DeleteUserRequest(username types.Username,
	requestId proto.RequestId) error {
	return s.deleteUserRequest(username, requestId)
}

func (s *Storer) ReadUpdates() (uint64, []proto.AllocationUpdateEntry, error) {
	return s.readUpdates()
}

func (s *Storer) ReadUsersQueue() ([]types.Username, error) {
	return s.readUsersQueue()
}

func (s *Storer) ReadUserQueue(username types.Username) (
	[]proto.RequestId, error) {
	return s.readUserQueue(username)
}

func (s *Storer) ReadUserRequest(username types.Username,
	requestId proto.RequestId) (proto.AllocateRequest, error) {
	return s.readUserRequest(username, requestId)
}

func (s *Storer) WriteUpdate(update proto.AllocationUpdateEntry,
	position uint64) error {
	return s.writeUpdate(update, position)
}

func (s *Storer) WriteUsersQueue(usernames []types.Username) error {
	return s.writeUsersQueue(usernames)
}

func (s *Storer) WriteUserQueue(username types.Username,
	requestIDs []proto.RequestId) error {
	return s.writeUserQueue(username, requestIDs)
}

func (s *Storer) WriteUserRequest(username types.Username,
	requestId proto.RequestId, request proto.AllocateRequest) error {
	return s.writeUserRequest(username, requestId, request)
}
