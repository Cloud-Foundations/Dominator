package fsstorer

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

const (
	updatesDirectory    = "updates"
	usersQueueDirectory = "users"
	usersQueueFile      = "users-queue"
	userQueueFile       = "queue"
)

func newStorer(topDir string, logger log.DebugLogger) (*Storer, error) {
	storer := &Storer{
		directoriesMade: make(map[string]struct{}),
		logger:          prefixlogger.New("allocator: fsstorer: ", logger),
		topDir:          topDir,
	}
	subdirs := []string{
		updatesDirectory,
		usersQueueDirectory,
	}
	if err := storer.mkSubdirs(subdirs); err != nil {
		return nil, err
	}
	return storer, nil
}

func readLines(filename string) ([]string, error) {
	lines, err := fsutil.LoadLines(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return lines, nil
}

func writeLines(filename string, lines []string) error {
	if len(lines) < 1 {
		err := os.Remove(filename)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	file, err := fsutil.CreateRenamingWriter(filename, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			file.Abort()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Abort()
		return err
	}
	return file.Close()
}

func (s *Storer) deleteUpdate(position uint64) error {
	filename := fmt.Sprintf("%s/%s/%d", s.topDir, updatesDirectory, position)
	return os.Remove(filename)
}

func (s *Storer) deleteUserRequest(username types.Username,
	requestId proto.RequestId) error {
	filename := filepath.Join(s.topDir, usersQueueDirectory,
		filepath.Clean(string(username)), filepath.Clean(string(requestId)))
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Storer) mkSubdir(subdir string) error {
	if _, ok := s.directoriesMade[subdir]; ok {
		return nil
	}
	err := os.MkdirAll(filepath.Join(s.topDir, subdir), fsutil.DirPerms)
	if err != nil {
		return err
	}
	s.directoriesMade[subdir] = struct{}{}
	return nil
}

func (s *Storer) mkSubdirs(subdirs []string) error {
	for _, subdir := range subdirs {
		if err := s.mkSubdir(subdir); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storer) readUpdates() (uint64, []proto.AllocationUpdateEntry, error) {
	startTime := time.Now()
	dirnames, err := fsutil.ReadDirnames(
		filepath.Join(s.topDir, updatesDirectory),
		true)
	if err != nil {
		return 0, nil, err
	}
	if len(dirnames) < 1 {
		return 0, nil, nil
	}
	positions := make([]uint64, 0, len(dirnames))
	for _, dirname := range dirnames {
		position, err := strconv.ParseUint(dirname, 10, 64)
		if err != nil {
			return 0, nil, err
		}
		positions = append(positions, position)
	}
	sort.Slice(positions, func(i, j int) bool {
		return positions[i] < positions[j]
	})
	updates := make([]proto.AllocationUpdateEntry, 0, len(positions))
	for index, position := range positions {
		if index > 0 && position != positions[index-1]+1 {
			return 0, nil, fmt.Errorf("position: %d followed by: %d",
				positions[index-1], position)
		}
		filename := fmt.Sprintf("%s/%s/%d",
			s.topDir, updatesDirectory, position)
		var update proto.AllocationUpdateEntry
		if err := json.ReadFromFile(filename, &update); err != nil {
			return 0, nil, err
		}
		updates = append(updates, update)
	}
	s.logger.Debugf(0, "loaded %d updates in %s\n",
		len(updates), format.Duration(time.Since(startTime)))
	return positions[0], updates, nil
}

func (s *Storer) readUsersQueue() ([]types.Username, error) {
	lines, err := readLines(filepath.Join(s.topDir, usersQueueFile))
	if err != nil {
		return nil, err
	}
	usernames := make([]types.Username, 0, len(lines))
	for _, line := range lines {
		usernames = append(usernames, types.Username(line))
	}
	return usernames, nil
}

func (s *Storer) readUserQueue(username types.Username) ([]proto.RequestId,
	error) {
	lines, err := readLines(filepath.Join(s.topDir, usersQueueDirectory,
		filepath.Clean(string(username)), userQueueFile))
	if err != nil {
		return nil, err
	}
	requestIDs := make([]proto.RequestId, 0, len(lines))
	for _, line := range lines {
		requestIDs = append(requestIDs, proto.RequestId(line))
	}
	return requestIDs, nil
}

func (s *Storer) readUserRequest(username types.Username,
	requestId proto.RequestId) (proto.AllocateRequest, error) {
	filename := filepath.Join(s.topDir, usersQueueDirectory,
		filepath.Clean(string(username)), filepath.Clean(string(requestId)))
	var request proto.AllocateRequest
	if err := json.ReadFromFile(filename, &request); err != nil {
		return proto.AllocateRequest{}, err
	}
	return request, nil
}

func (s *Storer) writeUpdate(update proto.AllocationUpdateEntry,
	position uint64) error {
	if err := s.mkSubdir(updatesDirectory); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s/%s/%d", s.topDir, updatesDirectory, position)
	buffer := &bytes.Buffer{}
	if err := json.WriteWithIndent(buffer, "    ", update); err != nil {
		return err
	}
	if allocation := update.Available; allocation != nil {
		s.logger.Debugf(0, "writing update[%d] allocation: %s\n",
			position, update.RequestId)
	}
	if deleted := update.Deleted; deleted != nil {
		s.logger.Debugf(0, "writing update[%d] deleted: %s\n",
			position, update.RequestId)
	}
	return fsutil.CopyToFileExclusive(filename, fsutil.PublicFilePerms, buffer,
		0)
}

func (s *Storer) writeUsersQueue(usernames []types.Username) error {
	s.logger.Debugf(0, "writing users queue with %d entries\n", len(usernames))
	stringUsernames := make([]string, 0, len(usernames))
	for _, username := range usernames {
		stringUsernames = append(stringUsernames, string(username))
	}
	return writeLines(filepath.Join(s.topDir, usersQueueFile), stringUsernames)
}

func (s *Storer) writeUserQueue(username types.Username,
	requestIDs []proto.RequestId) error {
	s.logger.Debugf(0, "writing user queue for: %s with %d entries\n",
		username, len(requestIDs))
	subdir := filepath.Join(usersQueueDirectory,
		filepath.Clean(string(username)))
	if err := s.mkSubdir(subdir); err != nil {
		return err
	}
	dirname := filepath.Join(s.topDir, subdir)
	stringRequestIDs := make([]string, 0, len(requestIDs))
	for _, requestId := range requestIDs {
		stringRequestIDs = append(stringRequestIDs, string(requestId))
	}
	return writeLines(filepath.Join(dirname, userQueueFile), stringRequestIDs)
}

func (s *Storer) writeUserRequest(username types.Username,
	requestId proto.RequestId, request proto.AllocateRequest) error {
	s.logger.Debugf(0, "writing user request for: %s, ID: %s\n",
		username, requestId)
	subdir := filepath.Join(usersQueueDirectory,
		filepath.Clean(string(username)))
	if err := s.mkSubdir(subdir); err != nil {
		return err
	}
	dirname := filepath.Join(s.topDir, subdir)
	filename := filepath.Join(dirname, filepath.Clean(string(requestId)))
	return json.WriteToFile(filename, fsutil.PublicFilePerms, "    ", request)
}
