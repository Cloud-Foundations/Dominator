package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	dm_proto "github.com/Cloud-Foundations/Dominator/proto/disruptionmanager"
	sub_proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

const (
	tagGroupIdentifier        = "DisruptionManagerGroupIdentifier"
	tagGroupMaximumDisrupting = "DisruptionManagerGroupMaximumDisrupting"
)

type disruptionManager struct {
	logger        log.DebugLogger
	maxDuration   time.Duration
	stateFilename string
	writeNotifier chan<- struct{}
	mutex         sync.Mutex                // Protect everything below.
	exportable    *groupListType            // nil if invalid.
	groups        map[string]*groupInfoType // Key: group identifier.
}

type groupInfoType struct {
	maxPermitted uint64
	permitted    map[string]time.Time // K: hostname, V: last request time.
	requested    map[string]time.Time // K: hostname, V: last request time.
}

type groupStatsType struct {
	Identifier string
	Permitted  []hostInfoType `json:",omitempty"`
	Requested  []hostInfoType `json:",omitempty"`
}

type hostInfoType struct {
	Hostname    string
	LastRequest time.Time
}

type groupListType struct {
	groups         []groupStatsType
	totalPermitted uint
	totalRequested uint
}

func newDisruptionManager(stateFilename string,
	maximimPermittedDuration time.Duration,
	logger log.DebugLogger) (*disruptionManager, error) {
	writeNotifier := make(chan struct{}, 1)
	var groupList groupListType
	dm := &disruptionManager{
		exportable:    &groupList,
		groups:        make(map[string]*groupInfoType),
		logger:        logger,
		maxDuration:   maximimPermittedDuration,
		stateFilename: stateFilename,
		writeNotifier: writeNotifier,
	}
	if err := json.ReadFromFile(stateFilename, &groupList.groups); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		for _, groupStats := range groupList.groups {
			group := newGroup()
			dm.groups[groupStats.Identifier] = group
			for _, host := range groupStats.Permitted {
				if _, ok := group.permitted[host.Hostname]; !ok {
					group.permitted[host.Hostname] = host.LastRequest
					groupList.totalPermitted++
				}
			}
			for _, host := range groupStats.Requested {
				if _, ok := group.permitted[host.Hostname]; !ok {
					group.requested[host.Hostname] = host.LastRequest
					groupList.totalRequested++
				}
			}
		}
	}
	go dm.expireLoop()
	go dm.writeLoop(writeNotifier)
	return dm, nil
}

func newGroup() *groupInfoType {
	return &groupInfoType{
		maxPermitted: 1,
		permitted:    make(map[string]time.Time),
		requested:    make(map[string]time.Time),
	}
}

func sortHostInfos(list []hostInfoType) {
	sort.SliceStable(list, func(left, right int) bool {
		return list[left].Hostname < list[right].Hostname
	})
}

func (dm *disruptionManager) cancel(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	logHostname := machine.Hostname
	if machine.Tags[tagGroupIdentifier] != "" {
		logHostname = machine.Tags[tagGroupIdentifier] + "/" + machine.Hostname
	}
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	group := dm.getGroup(machine.Tags[tagGroupIdentifier])
	var logMessage string
	if _, ok := group.permitted[machine.Hostname]; ok {
		invalidate = true
		// Move one host from Requested -> Permitted if possible.
		for hostname, lastRequest := range group.requested {
			group.permitted[hostname] = lastRequest
			delete(group.requested, hostname)
			logMessage = fmt.Sprintf(
				"%s: permitted->denied and %s: requested->permitted",
				logHostname, hostname)
			break
		}
		if logMessage == "" {
			logMessage = fmt.Sprintf("%s: permitted->denied", logHostname)
		}
		delete(group.permitted, machine.Hostname)
	}
	if _, ok := group.requested[machine.Hostname]; ok {
		invalidate = true
		if logMessage == "" {
			logMessage = fmt.Sprintf("%s: requested->denied", logHostname)
		}
		delete(group.requested, machine.Hostname)
	}
	return sub_proto.DisruptionStateDenied, logMessage, nil
}

func (dm *disruptionManager) check(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	logHostname := machine.Hostname
	if machine.Tags[tagGroupIdentifier] != "" {
		logHostname = machine.Tags[tagGroupIdentifier] + "/" + machine.Hostname
	}
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	group := dm.getGroup(machine.Tags[tagGroupIdentifier])
	if _, ok := group.permitted[machine.Hostname]; ok {
		return sub_proto.DisruptionStatePermitted, "", nil
	}
	lastRequestTime, previouslyRequested := group.requested[machine.Hostname]
	if !previouslyRequested {
		return sub_proto.DisruptionStateDenied, "", nil
	}
	if !group.canPermit(machine.Tags) {
		return sub_proto.DisruptionStateRequested, "", nil
	}
	// Previously requested and now there is room. W00t!
	invalidate = true
	group.permitted[machine.Hostname] = lastRequestTime
	delete(group.requested, machine.Hostname)
	return sub_proto.DisruptionStatePermitted,
		fmt.Sprintf("%s: requested->permitted", logHostname),
		nil
}

func (dm *disruptionManager) expireLoop() {
	for {
		for _, logLine := range dm.expireOnce() {
			dm.logger.Println(logLine)
		}
		time.Sleep(dm.maxDuration >> 6)
	}
}

func (dm *disruptionManager) expireOnce() []string {
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	expireBefore := time.Now().Add(-dm.maxDuration)
	var logLines []string
	for groupIdentifier, group := range dm.groups {
		for hostname, lastRequestTime := range group.permitted {
			if lastRequestTime.Before(expireBefore) {
				invalidate = true
				delete(group.permitted, hostname)
				logHostname := hostname
				if groupIdentifier != "" {
					logHostname = groupIdentifier + "/" + hostname
				}
				logLines = append(logLines,
					fmt.Sprintf("%s: permitted->denied", logHostname))
			}
		}
		for hostname, lastRequestTime := range group.requested {
			if lastRequestTime.Before(expireBefore) {
				invalidate = true
				delete(group.requested, hostname)
				logHostname := hostname
				if groupIdentifier != "" {
					logHostname = groupIdentifier + "/" + hostname
				}
				dm.logger.Printf("%s: requested->denied\n", logHostname)
			} else if uint64(len(group.permitted)) < group.maxPermitted {
				invalidate = true
				group.permitted[hostname] = lastRequestTime
				delete(group.requested, hostname)
				logHostname := hostname
				if groupIdentifier != "" {
					logHostname = groupIdentifier + "/" + hostname
				}
				logLines = append(logLines,
					fmt.Sprintf("%s: requested->permitted", logHostname))
			}
		}
	}
	return logLines
}

func (dm *disruptionManager) getGroup(groupIdentifier string) *groupInfoType {
	group := dm.groups[groupIdentifier]
	if group != nil {
		return group
	}
	group = newGroup()
	dm.groups[groupIdentifier] = group
	return group
}

func (dm *disruptionManager) getGroupList() *groupListType {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	if dm.exportable != nil {
		return dm.exportable
	}
	var groupList groupListType
	for groupIdentifier, group := range dm.groups {
		if len(group.permitted) < 1 && len(group.requested) < 1 {
			continue
		}
		groupStats := groupStatsType{
			Identifier: groupIdentifier,
		}
		for hostname, lastRequest := range group.permitted {
			groupStats.Permitted = append(groupStats.Permitted, hostInfoType{
				Hostname:    hostname,
				LastRequest: lastRequest,
			})
		}
		sortHostInfos(groupStats.Permitted)
		groupList.totalPermitted += uint(len(groupStats.Permitted))
		for hostname, lastRequest := range group.requested {
			groupStats.Requested = append(groupStats.Requested, hostInfoType{
				Hostname:    hostname,
				LastRequest: lastRequest,
			})
		}
		sortHostInfos(groupStats.Requested)
		groupList.totalRequested += uint(len(groupStats.Requested))
		groupList.groups = append(groupList.groups, groupStats)
	}
	groupList.sort()
	dm.exportable = &groupList
	return &groupList
}

func (dm *disruptionManager) processRequest(
	request dm_proto.DisruptionRequest) (*dm_proto.DisruptionResponse, error) {
	var err error
	var state sub_proto.DisruptionState
	var logMessage string
	switch request.Request {
	case sub_proto.DisruptionRequestCheck:
		state, logMessage, err = dm.check(request.MDB)
	case sub_proto.DisruptionRequestRequest:
		state, logMessage, err = dm.request(request.MDB)
	case sub_proto.DisruptionRequestCancel:
		state, logMessage, err = dm.cancel(request.MDB)
	default:
		err = fmt.Errorf("invalid request: %d", request.Request)
	}
	if err != nil {
		return nil, err
	}
	if logMessage != "" {
		dm.logger.Println(logMessage)
	}
	return &dm_proto.DisruptionResponse{Response: state}, nil
}

func (dm *disruptionManager) request(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	logHostname := machine.Hostname
	if machine.Tags[tagGroupIdentifier] != "" {
		logHostname = machine.Tags[tagGroupIdentifier] + "/" + machine.Hostname
	}
	dm.mutex.Lock()
	defer dm.unlockAndInvalidate(true)
	group := dm.getGroup(machine.Tags[tagGroupIdentifier])
	if _, ok := group.permitted[machine.Hostname]; ok {
		group.permitted[machine.Hostname] = time.Now()
		return sub_proto.DisruptionStatePermitted, "", nil
	}
	var logMessage string
	if group.canPermit(machine.Tags) {
		group.permitted[machine.Hostname] = time.Now()
		if _, ok := group.requested[machine.Hostname]; ok {
			logMessage = fmt.Sprintf("%s: requested->permitted",
				logHostname)
			delete(group.requested, machine.Hostname)
		} else {
			logMessage = fmt.Sprintf("%s: denied->permitted", logHostname)
		}
		return sub_proto.DisruptionStatePermitted, logMessage, nil
	}
	if _, ok := group.requested[machine.Hostname]; !ok {
		logMessage = fmt.Sprintf("%s: denied->requested", logHostname)
	}
	group.requested[machine.Hostname] = time.Now()
	return sub_proto.DisruptionStateRequested, logMessage, nil
}

func (dm *disruptionManager) unlockAndInvalidate(invalidate bool) {
	if invalidate {
		dm.exportable = nil
	}
	dm.mutex.Unlock()
	if !invalidate {
		return
	}
	select {
	case dm.writeNotifier <- struct{}{}:
	default:
	}
}

func (dm *disruptionManager) writeLoop(notifier <-chan struct{}) {
	for range notifier {
		if err := dm.writeOnce(); err != nil {
			dm.logger.Println(err)
		}
	}
}

func (dm *disruptionManager) writeOnce() error {
	groupList := dm.getGroupList()
	err := json.WriteToFile(dm.stateFilename, fsutil.PublicFilePerms, "    ",
		groupList.groups)
	if err != nil {
		dm.logger.Printf("error saving state: %s\n", err)
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

// canPermit returns true if the group can permit more disruption.
func (group *groupInfoType) canPermit(tgs tags.Tags) bool {
	maximum, err := strconv.ParseUint(tgs[tagGroupMaximumDisrupting], 10, 64)
	if err != nil || maximum < 1 {
		maximum = 1
	}
	group.maxPermitted = maximum
	return uint64(len(group.permitted)) < maximum
}

func (groupList *groupListType) sort() {
	sort.SliceStable(groupList.groups, func(left, right int) bool {
		return groupList.groups[left].Identifier <
			groupList.groups[right].Identifier
	})
}
