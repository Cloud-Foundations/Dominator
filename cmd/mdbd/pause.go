package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/proto/mdbserver"
)

const (
	pauseTableFilename = "pause-table.json"
)

func (pauseTable *pauseTableType) garbageCollectLoop(
	eventChannel chan<- struct{}, logger log.Logger) {
	for {
		if hostnames := pauseTable.garbageCollect(); len(hostnames) > 0 {
			logger.Printf("Updates automatically resuming for: %v\n", hostnames)
			select {
			case eventChannel <- struct{}{}:
			default:
			}
			pauseTable.write(logger)
		}
		time.Sleep(time.Second)
	}
}

// garbageCollect will remove expired entries from the pause table. It returns
// a list of machine names which have expired.
func (pauseTable *pauseTableType) garbageCollect() []string {
	var removedHostnames []string
	pauseTable.mutex.Lock()
	defer pauseTable.mutex.Unlock()
	if len(pauseTable.Machines) < 1 {
		return nil
	}
	for hostname, pauseData := range pauseTable.Machines {
		if time.Since(pauseData.Until) >= 0 {
			delete(pauseTable.Machines, hostname)
			removedHostnames = append(removedHostnames, hostname)
		}
	}
	return removedHostnames
}

func loadPauseTable() (*pauseTableType, error) {
	filename := filepath.Join(*stateDir, pauseTableFilename)
	pauseTable := pauseTableType{Machines: make(map[string]pauseDataType)}
	if err := json.ReadFromFile(filename, &pauseTable.Machines); err != nil {
		if os.IsNotExist(err) {
			return &pauseTable, nil
		}
		return nil, err
	}
	return &pauseTable, nil
}

func (pauseTable *pauseTableType) write(logger log.Logger) {
	filename := filepath.Join(*stateDir, pauseTableFilename)
	pauseTable.mutex.RLock()
	defer pauseTable.mutex.RUnlock()
	err := json.WriteToFile(filename, fsutil.PublicFilePerms, "    ",
		pauseTable.Machines)
	if err != nil {
		logger.Printf("error writing: %s\n")
	}
}

func (t *rpcType) pauseUpdates(conn *srpc.Conn,
	request mdbserver.PauseUpdatesRequest,
	reply *mdbserver.PauseUpdatesResponse) string {
	authInfo := conn.GetAuthInformation()
	if authInfo == nil {
		return "no authentication information"
	}
	duration := time.Until(request.Until)
	if duration <= time.Minute-time.Second {
		return "pause request too short"
	}
	if duration > *maximumPauseDuration {
		return "pause request too long"
	}
	currentMdb := t.currentMdb
	if currentMdb == nil {
		return "no MDB data"
	}
	machine, ok := currentMdb.table[request.Hostname]
	if !ok {
		return fmt.Sprintf("machine: %s not in MDB", request.Hostname)
	}
	ownerUsers := stringutil.ConvertListToMap(machine.OwnerUsers, false)
	haveAccess := authInfo.HaveMethodAccess
	if !haveAccess {
		if _, ok := ownerUsers[authInfo.Username]; ok {
			haveAccess = true
		}
	}
	if !haveAccess {
		for _, ownerGroup := range machine.OwnerGroups {
			if _, ok := authInfo.GroupList[ownerGroup]; ok {
				haveAccess = true
				break
			}
		}
	}
	if !haveAccess {
		return fmt.Sprintf("you do not have ownership of: %s", request.Hostname)
	}
	if err := t.pauseUpdatesTakeLock(request, authInfo.Username); err != "" {
		return err
	}
	t.logger.Printf("PauseUpdates(%s): machine: %s until: %s (for %s)\n",
		authInfo.Username, request.Hostname, request.Until,
		format.Duration(duration))
	select {
	case t.eventChannel <- struct{}{}:
	default:
	}
	t.pauseTable.write(t.logger)
	return ""
}

func (t *rpcType) pauseUpdatesTakeLock(request mdbserver.PauseUpdatesRequest,
	username string) string {
	t.pauseTable.mutex.Lock()
	defer t.pauseTable.mutex.Unlock()
	var userCount uint
	for _, pauseData := range t.pauseTable.Machines {
		if pauseData.Username == username {
			userCount++
		}
	}
	if userCount > *maximumPausedMachinesPerUser {
		return "you have too many paused machines"
	}
	if pauseData, ok := t.pauseTable.Machines[request.Hostname]; ok {
		if username != pauseData.Username {
			return fmt.Sprintf(
				"machine: %s already paused by: %s for: %s until: %s",
				request.Hostname, pauseData.Username, pauseData.Reason,
				pauseData.Until)
		}
	}
	t.pauseTable.Machines[request.Hostname] = pauseDataType{
		Reason:   request.Reason,
		Until:    request.Until,
		Username: username,
	}
	return ""
}

func (t *rpcType) resumeUpdates(conn *srpc.Conn,
	request mdbserver.ResumeUpdatesRequest,
	reply *mdbserver.ResumeUpdatesResponse) string {
	currentMdb := t.currentMdb
	if currentMdb == nil {
		return "no MDB data"
	}
	if _, ok := currentMdb.table[request.Hostname]; !ok {
		return fmt.Sprintf("machine: %s not in MDB", request.Hostname)
	}
	username := conn.Username()
	if err := t.resumeUpdatesTakeLock(request, username); err != "" {
		return err
	}
	t.logger.Printf("ResumeUpdates(%s): machine: %s\n",
		username, request.Hostname)
	select {
	case t.eventChannel <- struct{}{}:
	default:
	}
	t.pauseTable.write(t.logger)
	return ""
}

func (t *rpcType) resumeUpdatesTakeLock(request mdbserver.ResumeUpdatesRequest,
	username string) string {
	t.pauseTable.mutex.Lock()
	defer t.pauseTable.mutex.Unlock()
	if pauseData, ok := t.pauseTable.Machines[request.Hostname]; !ok {
		return fmt.Sprintf("machine: %s is not paused", request.Hostname)
	} else if username != pauseData.Username {
		return fmt.Sprintf("machine: %s is paused by: %s",
			request.Hostname, pauseData.Username)
	}
	delete(t.pauseTable.Machines, request.Hostname)
	return ""
}
