package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mdb"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	sub_proto "github.com/Cloud-Foundations/Dominator/proto/sub"
)

const (
	tagGroupIdentifier               = "DisruptionManagerGroupIdentifier"
	tagGroupMaximumDisrupting        = "DisruptionManagerGroupMaximumDisrupting"
	tagDisruptionManagerReadyTimeout = "DisruptionManagerReadyTimeout"
	tagDisruptionManagerReadyUrl     = "DisruptionManagerReadyUrl"
)

type disruptionManager struct {
	logger              log.DebugLogger
	maxDuration         time.Duration
	stateFilename       string
	recalculateNotifier chan<- struct{}
	writeNotifier       chan<- struct{}
	mutex               sync.Mutex                // Protect everything below.
	exportable          *groupListType            // nil if invalid.
	groups              map[string]*groupInfoType // Key: group identifier.
}

type groupInfoType struct {
	maxPermitted uint64
	permitted    map[string]time.Time     // K: hostname, V: last request time.
	requested    map[string]time.Time     // K: hostname, V: last request time.
	waiting      map[string]*waitDataType // K: hostname.
}

type groupStatsType struct {
	Identifier string
	Permitted  []hostInfoType `json:",omitempty"`
	Requested  []hostInfoType `json:",omitempty"`
	Waiting    []waitInfoType `json:",omitempty"`
}

type hostInfoType struct {
	Hostname    string
	LastRequest time.Time `json:",omitempty"`
	waitInfoType
}

type groupListType struct {
	groups         []groupStatsType
	totalPermitted uint
	totalRequested uint
	totalWaiting   uint
}

type waitDataType struct {
	finished     bool
	ReadyTimeout time.Time `json:",omitempty"`
	ReadyUrl     string    `json:",omitempty"`
}

type waitInfoType struct {
	Hostname string
	waitDataType
}

// Returns nil if the remote hostname matches the MDB hostname, else an error.
func hostAccessCheck(remoteAddr, mdbHostname string) error {
	remoteIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return err
	}
	if remoteIP == mdbHostname {
		return nil
	}
	mdbIPs, err := net.LookupHost(mdbHostname)
	if err != nil {
		return err
	}
	for _, mdbIP := range mdbIPs {
		if remoteIP == mdbIP {
			return nil
		}
	}
	return fmt.Errorf("%s not permitted", mdbHostname)
}

func makeGroupText(groupIdentifier string) string {
	if groupIdentifier == "" {
		return "global group"
	} else {
		return `group="` + groupIdentifier + `"`
	}
}

func sendNotification(notifier chan<- struct{}) {
	select {
	case notifier <- struct{}{}:
	default:
	}
}

func sortHostInfos(list []hostInfoType) {
	sort.SliceStable(list, func(left, right int) bool {
		return list[left].Hostname < list[right].Hostname
	})
}

func sortWaitInfos(list []waitInfoType) {
	sort.SliceStable(list, func(left, right int) bool {
		return list[left].Hostname < list[right].Hostname
	})
}

func newDisruptionManager(stateFilename string,
	maximumPermittedDuration time.Duration,
	logger log.DebugLogger) (*disruptionManager, error) {
	recalculateNotifier := make(chan struct{}, 1)
	writeNotifier := make(chan struct{}, 1)
	var groupList groupListType
	dm := &disruptionManager{
		exportable:          &groupList,
		groups:              make(map[string]*groupInfoType),
		logger:              logger,
		maxDuration:         maximumPermittedDuration,
		stateFilename:       stateFilename,
		recalculateNotifier: recalculateNotifier,
		writeNotifier:       writeNotifier,
	}
	if stateFilename != "" {
		err := json.ReadFromFile(stateFilename, &groupList.groups)
		if err != nil {
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
				for _, host := range groupStats.Waiting {
					if _, ok := group.waiting[host.Hostname]; !ok {
						if !host.ReadyTimeout.IsZero() &&
							time.Until(host.ReadyTimeout) > 0 {
							group.waiting[host.Hostname] = &host.waitDataType
							go host.waitDataType.wait(recalculateNotifier,
								host.Hostname,
								makeGroupText(groupStats.Identifier), logger)
							groupList.totalWaiting++
						}
					}
				}
			}
			go dm.writeLoop(writeNotifier)
		}
	}
	go dm.recalculateLoop(recalculateNotifier)
	return dm, nil
}

func (dm *disruptionManager) cancel(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	waitData := makeWaitData(machine, dm.logger)
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	group, groupText := dm.getGroup(machine)
	var logMessage string
	if _, ok := group.permitted[machine.Hostname]; ok {
		invalidate = true
		if waitData != nil {
			group.waiting[machine.Hostname] = waitData
			go waitData.wait(dm.recalculateNotifier, machine.Hostname,
				groupText, dm.logger)
			logMessage = fmt.Sprintf("%s: permitted->denied/waiting (%s)",
				machine.Hostname, groupText)
		} else {
			// Move one host from Requested -> Permitted if possible.
			for hostname, lastRequest := range group.requested {
				group.permitted[hostname] = lastRequest
				delete(group.requested, hostname)
				logMessage = fmt.Sprintf(
					"%s: permitted->denied and %s: requested->permitted (%s)",
					machine.Hostname, hostname, groupText)
				break
			}
			if logMessage == "" {
				logMessage = fmt.Sprintf("%s: permitted->denied (%s)",
					machine.Hostname, groupText)
			}
		}
		delete(group.permitted, machine.Hostname)
	}
	if _, ok := group.requested[machine.Hostname]; ok {
		invalidate = true
		if logMessage == "" {
			logMessage = fmt.Sprintf("%s: requested->denied (%s)",
				machine.Hostname, groupText)
		}
		delete(group.requested, machine.Hostname)
	}
	return sub_proto.DisruptionStateDenied, logMessage, nil
}

func (dm *disruptionManager) check(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	group, groupText := dm.getGroup(machine)
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
		fmt.Sprintf("%s: requested->permitted (%s)",
			machine.Hostname, groupText),
		nil
}

func (dm *disruptionManager) getGroup(machine mdb.Machine) (
	*groupInfoType, string) {
	var groupIdentifier string
	if id, ok := machine.Tags[tagGroupIdentifier]; ok {
		groupIdentifier = id
	} else {
		groupIdentifier = path.Dir(machine.RequiredImage)
	}
	group := dm.groups[groupIdentifier]
	if group == nil {
		group = newGroup()
		dm.groups[groupIdentifier] = group
	}
	return group, makeGroupText(groupIdentifier)
}

func (dm *disruptionManager) getGroupList() *groupListType {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	if dm.exportable != nil {
		return dm.exportable
	}
	var groupList groupListType
	for groupIdentifier, group := range dm.groups {
		if len(group.permitted) < 1 &&
			len(group.requested) < 1 &&
			len(group.waiting) < 1 {
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
		for hostname, waitData := range group.waiting {
			groupStats.Waiting = append(groupStats.Waiting, waitInfoType{
				Hostname:     hostname,
				waitDataType: *waitData,
			})
		}
		sortWaitInfos(groupStats.Waiting)
		groupList.totalWaiting += uint(len(groupStats.Waiting))
		groupList.groups = append(groupList.groups, groupStats)
	}
	groupList.sort()
	dm.exportable = &groupList
	return &groupList
}

func (dm *disruptionManager) recalculateLoop(notifier <-chan struct{}) {
	for {
		for _, logLine := range dm.recalculateOnce() {
			dm.logger.Println(logLine)
		}
		timer := time.NewTimer(dm.maxDuration >> 6)
		select {
		case <-notifier:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
	}
}

func (dm *disruptionManager) recalculateOnce() []string {
	var invalidate bool
	dm.mutex.Lock()
	defer func() {
		dm.unlockAndInvalidate(invalidate)
	}()
	expireBefore := time.Now().Add(-dm.maxDuration)
	var logLines []string
	for groupIdentifier, group := range dm.groups {
		groupText := makeGroupText(groupIdentifier)
		for hostname, waitData := range group.waiting {
			if waitData.finished {
				invalidate = true
				delete(group.waiting, hostname)
			}
		}
		for hostname, lastRequestTime := range group.permitted {
			if lastRequestTime.Before(expireBefore) {
				invalidate = true
				delete(group.permitted, hostname)
				logLines = append(logLines,
					fmt.Sprintf("%s: permitted/expired->denied (%s)",
						hostname, groupText))
			}
		}
		for hostname, lastRequestTime := range group.requested {
			if lastRequestTime.Before(expireBefore) {
				invalidate = true
				delete(group.requested, hostname)
				dm.logger.Printf("%s: requested/expired->denied (%s)\n",
					hostname, groupText)
			} else if group.canPermit(nil) {
				invalidate = true
				group.permitted[hostname] = lastRequestTime
				delete(group.requested, hostname)
				logLines = append(logLines,
					fmt.Sprintf("%s: requested->permitted (%s)",
						hostname, groupText))
			}
		}
	}
	return logLines
}

func (dm *disruptionManager) request(machine mdb.Machine) (
	sub_proto.DisruptionState, string, error) {
	dm.mutex.Lock()
	defer dm.unlockAndInvalidate(true)
	group, groupText := dm.getGroup(machine)
	if _, ok := group.permitted[machine.Hostname]; ok {
		group.permitted[machine.Hostname] = time.Now()
		return sub_proto.DisruptionStatePermitted, "", nil
	}
	var logMessage string
	if group.canPermit(machine.Tags) {
		group.permitted[machine.Hostname] = time.Now()
		if _, ok := group.requested[machine.Hostname]; ok {
			logMessage = fmt.Sprintf("%s: requested->permitted (%s)",
				machine.Hostname, groupText)
			delete(group.requested, machine.Hostname)
		} else {
			logMessage = fmt.Sprintf("%s: denied->permitted (%s)",
				machine.Hostname, groupText)
		}
		return sub_proto.DisruptionStatePermitted, logMessage, nil
	}
	if _, ok := group.requested[machine.Hostname]; !ok {
		logMessage = fmt.Sprintf("%s: denied->requested (%s)",
			machine.Hostname, groupText)
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
	sendNotification(dm.writeNotifier)
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
	return uint64(len(group.permitted)+len(group.waiting)) < maximum
}

func newGroup() *groupInfoType {
	return &groupInfoType{
		maxPermitted: 1,
		permitted:    make(map[string]time.Time),
		requested:    make(map[string]time.Time),
		waiting:      make(map[string]*waitDataType),
	}
}

func (groupList *groupListType) sort() {
	sort.SliceStable(groupList.groups, func(left, right int) bool {
		return groupList.groups[left].Identifier <
			groupList.groups[right].Identifier
	})
}

func makeWaitData(machine mdb.Machine, logger log.Logger) *waitDataType {
	var retval *waitDataType
	waitData := waitDataType{}
	if value, ok := machine.Tags[tagDisruptionManagerReadyTimeout]; ok {
		if readyTimeout, err := time.ParseDuration(value); err != nil {
			logger.Printf("%s: error parsing [%s]=%s\n",
				machine.Hostname, tagDisruptionManagerReadyTimeout, value)
			return nil
		} else {
			waitData.ReadyTimeout = time.Now().Add(readyTimeout)
			retval = &waitData
		}
	}
	if value, ok := machine.Tags[tagDisruptionManagerReadyUrl]; ok {
		tmpl, err := template.New("").Parse(value)
		if err != nil {
			logger.Printf("%s: error parsing [%s]=%s\n",
				machine.Hostname, tagDisruptionManagerReadyUrl, value)
			return nil
		}
		builder := &strings.Builder{}
		if err := tmpl.Execute(builder, machine); err != nil {
			logger.Printf("%s: error executing [%s]=%s\n",
				machine.Hostname, tagDisruptionManagerReadyUrl, value)
			return nil
		}
		if waitData.ReadyTimeout.IsZero() {
			waitData.ReadyTimeout = time.Now().Add(15 * time.Minute)
		}
		waitData.ReadyUrl = builder.String()
		retval = &waitData
	}
	return retval
}

func (wd *waitDataType) wait(recalculateNotifier chan<- struct{},
	hostname, groupText string, logger log.DebugLogger) {
	maxDelay := time.Until(wd.ReadyTimeout)
	if wd.ReadyUrl == "" { // Simple delay.
		time.Sleep(maxDelay)
		wd.finished = true
		logger.Printf("%s: ready delay completed (%s)\n", hostname, groupText)
		sendNotification(recalculateNotifier)
		return
	}
	maxInterval := maxDelay >> 6
	if maxInterval > 5*time.Minute {
		maxInterval = 5 * time.Minute
	}
	sleeper := backoffdelay.NewExponential(maxInterval>>4, maxInterval, 2)
	for ; time.Until(wd.ReadyTimeout) > 0; sleeper.Sleep() {
		resp, err := http.Get(wd.ReadyUrl)
		if err != nil {
			logger.Debugf(1, "%s: %s\n", wd.ReadyUrl, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logger.Debugf(1, "%s: %s\n", wd.ReadyUrl, resp.Status)
			continue
		}
		wd.finished = true
		logger.Printf("%s: ready (%s)\n", hostname, groupText)
		sendNotification(recalculateNotifier)
		return
	}
	wd.finished = true
	logger.Printf("%s: ready check timed out (%s)\n", hostname, groupText)
	sendNotification(recalculateNotifier)
}
