package allocator

import (
	"fmt"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/list"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/net/util"
	"github.com/Cloud-Foundations/Dominator/lib/queue"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	"github.com/Cloud-Foundations/tricorder/go/tricorder"
	"github.com/Cloud-Foundations/tricorder/go/tricorder/units"
)

type manager struct {
	allocateRequestChannel  <-chan allocateRequestType
	allocations             map[fm_proto.RequestId]*allocationType
	cancelAllocationChannel <-chan cancelAllocationType
	closeNotifierChannel    <-chan <-chan fm_proto.AllocationUpdate
	deleted                 map[fm_proto.RequestId]*deletedType
	getRequestChannel       <-chan getRequestType
	heartbeatChannel        chan<- struct{}
	hypervisorDatas         map[string]fm_proto.HypervisorData // Key: Hostname.
	lastIdTime              string
	lastSequence            uint
	listAllocationsChannel  <-chan chan<- []allocationEntryType
	listQueueChannel        <-chan chan<- []fm_proto.AllocateRequestEntry
	machines                map[string]*fm_proto.Machine // Key: Hostname.
	notifiers               map[<-chan fm_proto.AllocationUpdate]chan<- fm_proto.AllocationUpdate
	options                 Options
	myIP                    net.IP
	params                  Params
	topology                *topology.Topology
	topologyChannel         <-chan *topology.Topology
	updateChannel           chan<- fm_proto.AllocationUpdateEntry
	updateQueue             queue.BroadcastQueue[fm_proto.AllocationUpdateEntry]
	userQueues              map[types.Username]*list.UniqueList[fm_proto.RequestId]
	usersQueue              *list.UniqueList[types.Username]
	vmToHypervisor          map[string]string // Key: VM IP, Value: Hostname
	vms                     map[string]*hyper_proto.VmInfo
	waitingRequestsById     map[fm_proto.RequestId]*requestType
}

type allocationType struct {
	allocation    *fm_proto.Allocation
	indexToVmIp   map[int]string // Key: VM index, value: VM IP address.
	request       *fm_proto.AllocateRequest
	vmHypervisors []string       // Hypervisor hostname.
	vmIpToIndex   map[string]int // Key: VM IP address, value: VM index.
	username      types.Username
}

type deletedType struct {
	allocation *fm_proto.Allocation
	deleted    *fm_proto.DeletedAllocation
	request    *fm_proto.AllocateRequest
	username   types.Username
}

type requestType struct {
	*fm_proto.AllocateRequest
	username types.Username
}

// checkVmMatchesSpec returns true if the VM matches the allocation spec.
func checkVmMatchesSpec(vmInfo *hyper_proto.VmInfo,
	vmSpec *fm_proto.VmAllocationSpecification) bool {
	if vmSpec.MemoryInMiB != vmInfo.MemoryInMiB {
		return false
	}
	if vmSpec.MilliCPUs != vmInfo.MilliCPUs {
		return false
	}
	if len(vmSpec.NetworkInterfaces) != len(vmInfo.SecondarySubnetIDs)+1 {
		return false
	}
	if vmSpec.NetworkInterfaces[0].SubnetId != vmInfo.SubnetId {
		return false
	}
	for index, subnetId := range vmInfo.SecondarySubnetIDs {
		if subnetId != vmSpec.NetworkInterfaces[index+1].SubnetId {
			return false
		}
	}
	if len(vmSpec.Volumes) != len(vmInfo.Volumes) {
		return false
	}
	return true
}

func clearTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func watchHeartbeat(timeout time.Duration, heartbeatChannel <-chan struct{},
	heartbeatTimestamp *time.Time, heartbeatStopped *bool,
	lastHeartbeatLostTime *time.Time, logger log.Logger) {
	timer := time.NewTimer(timeout)
	for {
		select {
		case <-timer.C:
			logger.Println("manager heartbeat stopped")
			*heartbeatStopped = true
			*lastHeartbeatLostTime = time.Now()
		case <-heartbeatChannel:
			if *heartbeatStopped {
				logger.Println("manager heartbeat resumed")
				*heartbeatStopped = false
			}
			clearTimer(timer)
			timer.Reset(timeout)
			*heartbeatTimestamp = time.Now()
		}
	}
}

func newManager(options Options, params Params) (*Manager, error) {
	params.Logger = prefixlogger.New("allocator: ", params.Logger)
	if params.heartbeatTimeout <= 0 {
		params.heartbeatTimeout = time.Minute
	}
	if params.managerInterval <= 0 {
		params.managerInterval = time.Second
	}
	allocateRequestChannel := make(chan allocateRequestType, 1)
	cancelAllocationChannel := make(chan cancelAllocationType, 1)
	closeNotifierChannel := make(chan (<-chan fm_proto.AllocationUpdate), 1)
	getRequestChannel := make(chan getRequestType, 1)
	heartbeatChannel := make(chan struct{}, 1)
	listAllocationsChannel := make(chan chan<- []allocationEntryType, 1)
	listQueueChannel := make(chan chan<- []fm_proto.AllocateRequestEntry, 1)
	readyChannel := make(chan struct{}, 1)
	readyChannel <- struct{}{}
	topologyChannel := make(chan *topology.Topology, 1)
	var lastHeartbeatTime time.Time
	myIP, err := util.GetMyIP()
	if err != nil {
		return nil, err
	}
	myIP = util.ShrinkIP(myIP)
	startPosition, updates, err := params.Storer.ReadUpdates()
	if err != nil {
		return nil, err
	}
	updateQueue, updateChannel, removeChannel := queue.NewBroadcastQueue[fm_proto.AllocationUpdateEntry](
		startPosition, options.MaximumQueueSize)
	managerPublic := &Manager{
		allocateRequestChannel:  allocateRequestChannel,
		cancelAllocationChannel: cancelAllocationChannel,
		closeNotifierChannel:    closeNotifierChannel,
		getRequestChannel:       getRequestChannel,
		listAllocationsChannel:  listAllocationsChannel,
		listQueueChannel:        listQueueChannel,
		ready:                   readyChannel,
		topologyChannel:         topologyChannel,
		updateQueue:             updateQueue,
	}
	managerPublic.httpSetup()
	managerInternal := &manager{
		allocateRequestChannel:  allocateRequestChannel,
		allocations:             make(map[fm_proto.RequestId]*allocationType),
		cancelAllocationChannel: cancelAllocationChannel,
		closeNotifierChannel:    closeNotifierChannel,
		deleted:                 make(map[fm_proto.RequestId]*deletedType),
		getRequestChannel:       getRequestChannel,
		heartbeatChannel:        heartbeatChannel,
		listAllocationsChannel:  listAllocationsChannel,
		listQueueChannel:        listQueueChannel,
		notifiers: make(
			map[<-chan fm_proto.AllocationUpdate]chan<- fm_proto.AllocationUpdate),
		options:         options,
		myIP:            myIP,
		params:          params,
		topologyChannel: topologyChannel,
		updateChannel:   updateChannel,
		updateQueue:     updateQueue,
		userQueues: make(
			map[types.Username]*list.UniqueList[fm_proto.RequestId]),
		usersQueue:          list.NewUnique[types.Username](),
		waitingRequestsById: make(map[fm_proto.RequestId]*requestType),
	}
	managerInternal.initialiseResources()
	if err := managerInternal.loadQueues(updates); err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		managerPublic.active = true
	}
	nextExpiration, err := managerInternal.expireRequests(true)
	if err != nil {
		return nil, err
	}
	go managerInternal.manage(nextExpiration, removeChannel)
	go watchHeartbeat(params.heartbeatTimeout, heartbeatChannel,
		&lastHeartbeatTime, &managerPublic.lostHeartbeat,
		&managerPublic.lastLostHeartbeatTime, params.Logger)
	tricorder.RegisterMetric("allocator/last-heartbeat-time",
		&lastHeartbeatTime, units.None,
		"last manager heartbeat timestamp")
	// Injecting updates has an implicit dependency (in the manager goroutine)
	// on the topology being loaded, inject in the background so that this
	// function returns and topology data starts being loaded.
	go func() {
		for _, update := range updates {
			updateChannel <- update
		}
		params.Logger.Println("updates injected and ready for use")
		<-readyChannel
	}()
	return managerPublic, nil
}

func (m *Manager) allocate(authInfo *srpc.AuthInformation,
	request fm_proto.AllocateRequest) (fm_proto.AllocateResponse, error) {
	m.waitForReady()
	if !m.active {
		m.active = true
	}
	channel := make(chan fm_proto.AllocateResponse, 1)
	req := allocateRequestType{
		request:  request,
		response: channel,
		username: types.Username(authInfo.Username),
	}
	m.allocateRequestChannel <- req
	return <-channel, nil
}

func (m *Manager) cancelAllocation(authInfo *srpc.AuthInformation,
	requestId fm_proto.RequestId) error {
	m.waitForReady()
	channel := make(chan error, 1)
	req := cancelAllocationType{
		authInfo:  authInfo,
		requestId: requestId,
		response:  channel,
	}
	m.cancelAllocationChannel <- req
	return <-channel
}

func (m *Manager) closeUpdateChannel(channel <-chan fm_proto.AllocationUpdate) {
	m.waitForReady()
	m.closeNotifierChannel <- channel
}

func (m *Manager) getRequest(requestId fm_proto.RequestId) *requestInfo {
	m.waitForReady()
	channel := make(chan *requestInfo, 1)
	// TODO(rgooch): a "public" structure should be returned here.
	m.getRequestChannel <- getRequestType{
		requestId: requestId,
		response:  channel,
	}
	return <-channel
}

func (m *Manager) listAllocations() []allocationEntryType {
	m.waitForReady()
	channel := make(chan []allocationEntryType, 1)
	m.listAllocationsChannel <- channel
	return <-channel
}

func (m *Manager) listQueue() []fm_proto.AllocateRequestEntry {
	m.waitForReady()
	channel := make(chan []fm_proto.AllocateRequestEntry, 1)
	m.listQueueChannel <- channel
	return <-channel
}

func (m *Manager) waitForReady() {
	m.ready <- struct{}{}
	<-m.ready
}

func (m *manager) cancelAllocation(authInfo *srpc.AuthInformation,
	requestId fm_proto.RequestId) error {
	m.params.Logger.Debugf(0, "cancelling request: %s\n", requestId)
	var requestEntry *fm_proto.AllocateRequest
	var username types.Username
	deletedEntry := fm_proto.DeletedAllocation{
		Reason: fm_proto.AllocationRequestCancelled,
	}
	if request := m.waitingRequestsById[requestId]; request != nil {
		if err := m.dequeue(authInfo, requestId); err != nil {
			return err
		}
		username = request.username
		m.deleted[requestId] = &deletedType{
			deleted:  &deletedEntry,
			request:  request.AllocateRequest,
			username: request.username,
		}
		requestEntry = request.AllocateRequest
	} else if allocation := m.allocations[requestId]; allocation != nil {
		username = allocation.username
		m.deleted[requestId] = &deletedType{
			allocation: allocation.allocation,
			deleted:    &deletedEntry,
			request:    allocation.request,
			username:   allocation.username,
		}
		delete(m.allocations, requestId)
	} else {
		return fmt.Errorf("cancelAllocation(): unknown request ID: %s",
			requestId)
	}
	return m.sendDeleted(requestId, requestEntry, deletedEntry, username)
}

func (m *manager) expireRequests(loading bool) (time.Time, error) {
	startTime := time.Now()
	nextExpiration := startTime.Add(15 * time.Minute)
	var prefix string
	if loading {
		prefix = "already "
	}
	var rewriteUsersList bool
	// Loop over outstanding requests, looking for those which have exceeded the
	// requested deadline.
	for requestId, request := range m.waitingRequestsById {
		deletedEntry := fm_proto.DeletedAllocation{
			Reason: fm_proto.AllocationRequestExpired,
		}
		if request.Deadline.IsZero() {
			continue
		}
		expired := time.Since(request.Deadline)
		if expired < 0 {
			if request.Deadline.Before(nextExpiration) {
				nextExpiration = request.Deadline
			}
			continue
		}
		deletedEntry.Reason = fm_proto.AllocationRequestExpired
		m.params.Logger.Printf(
			"deleting %sexpired request: %s (%s ago)\n",
			prefix, requestId, format.Duration(expired))
		userQueue := m.userQueues[request.username]
		if userQueue == nil {
			return nextExpiration,
				fmt.Errorf("expireRequests(): no user for request ID: %s",
					requestId)
		}
		userEntry := userQueue.Get(requestId)
		if userEntry == nil {
			return nextExpiration,
				fmt.Errorf(
					"expireRequests(): no queue entry for request ID: %s",
					requestId)
		}
		err := m.params.Storer.DeleteUserRequest(request.username, requestId)
		if err != nil {
			return nextExpiration, err
		}
		userEntry.Remove()
		if err := m.writeUserQueue(request.username, userQueue); err != nil {
			return nextExpiration, nil
		}
		if userQueue.Length() < 1 {
			delete(m.userQueues, request.username)
			m.usersQueue.Remove(request.username)
			rewriteUsersList = true
		}
		err = m.sendDeleted(requestId, request.AllocateRequest, deletedEntry,
			request.username)
		if err != nil {
			m.params.Logger.Println(err)
			return nextExpiration, nil
		}
		m.deleted[requestId] = &deletedType{
			deleted:  &deletedEntry,
			request:  request.AllocateRequest,
			username: request.username,
		}
		delete(m.waitingRequestsById, requestId)
		nextExpiration = startTime
	}
	if rewriteUsersList {
		if err := m.writeUsersQueue(); err != nil {
			return nextExpiration, err
		}
	}
	// Loop over allocations, looking for those where the resource creation
	// deadline has been exceeded.
	for requestId, allocation := range m.allocations {
		deletedEntry := fm_proto.DeletedAllocation{
			Reason: fm_proto.AllocationRequestCreateTimeout,
		}
		expired := time.Since(allocation.allocation.CreateDeadline)
		if expired < 0 {
			if allocation.allocation.CreateDeadline.Before(nextExpiration) {
				nextExpiration = allocation.allocation.CreateDeadline
			}
			continue
		}
		m.params.Logger.Printf(
			"deleting %sexpired allocation: %s (%s ago)\n",
			prefix, requestId, format.Duration(expired))
		err := m.sendDeleted(requestId, allocation.request, deletedEntry,
			allocation.username)
		if err != nil {
			m.params.Logger.Println(err)
			return nextExpiration, nil
		}
		m.deleted[requestId] = &deletedType{
			allocation: allocation.allocation,
			deleted:    &deletedEntry,
			request:    allocation.request,
			username:   allocation.username,
		}
		delete(m.allocations, requestId)
		nextExpiration = startTime
	}
	return nextExpiration, nil
}

func (m *manager) initialiseResources() {
	m.machines = make(map[string]*fm_proto.Machine)
	m.hypervisorDatas = make(map[string]fm_proto.HypervisorData)
	m.vmToHypervisor = make(map[string]string)
	m.vms = make(map[string]*hyper_proto.VmInfo)
}

func (m *manager) listAllocations() []allocationEntryType {
	var allocations []allocationEntryType
	for requestId, allocation := range m.allocations {
		allocations = append(allocations, allocationEntryType{
			allocation: allocation.allocation,
			requestId:  requestId,
			username:   allocation.username,
		})
	}
	return allocations
}

func (m *manager) manage(nextExpiration time.Time,
	removeChannel <-chan queue.BroadcastEntry[fm_proto.AllocationUpdateEntry]) {
	latencyBucketer := tricorder.NewGeometricBucketer(1e-3, 10)
	recalculateTimeDistribution := latencyBucketer.NewCumulativeDistribution()
	tricorder.RegisterMetric("allocator/recalculate-time",
		recalculateTimeDistribution, units.Millisecond, "recalculation time")
	updateChannel := m.params.UpdateChannelMaker.MakeUpdateChannel(
		fm_proto.GetUpdatesRequest{IgnoreMissingLocalTags: true})
	var resetResources bool
	expireTimer := time.NewTimer(time.Until(nextExpiration))
	timer := time.NewTimer(m.params.managerInterval)
	for {
		// The removeChannel has a buffer length of 1 and if it fills the
		// broadcast queue will be stuck waiting to drain the channel and thus
		// this goroutine will lock up if it tries to send to the queue. Ensure
		// that remove messages are all processed before doing anything else.
		select {
		case entry := <-removeChannel:
			if err := m.params.Storer.DeleteUpdate(entry.Position); err != nil {
				m.params.Logger.Println(err)
			}
			delete(m.deleted, entry.Value.RequestId)
			continue
		default:
		}
		var checkExpirations, recalculate bool
		select {
		case request := <-m.allocateRequestChannel:
			response, err := m.enqueue(request.request, request.username)
			if err != nil {
				request.response <- fm_proto.AllocateResponse{
					Error: err.Error()}
			} else {
				request.response <- *response
				checkExpirations = true
				recalculate = true
				m.params.Logger.Debugf(1,
					"received allocation request, issued ID: %s, UpdatePosition: %d\n",
					response.RequestId, response.UpdatePosition)
			}
		case request := <-m.cancelAllocationChannel:
			request.response <- m.cancelAllocation(request.authInfo,
				request.requestId)
			recalculate = true
		case notifier := <-m.closeNotifierChannel:
			delete(m.notifiers, notifier)
		case request := <-m.getRequestChannel:
			if alloc := m.allocations[request.requestId]; alloc != nil {
				request.response <- &requestInfo{
					Allocation: alloc.allocation,
					Request:    alloc.request,
					Username:   alloc.username,
				}
				break
			}
			if req := m.waitingRequestsById[request.requestId]; req != nil {
				request.response <- &requestInfo{
					Request:  req.AllocateRequest,
					Username: req.username,
				}
				break
			}
			if del := m.deleted[request.requestId]; del != nil {
				request.response <- &requestInfo{
					Allocation: del.allocation,
					Request:    del.request,
					Deleted:    del.deleted,
					Username:   del.username,
				}
				break
			}
			request.response <- nil
		case channel := <-m.listAllocationsChannel:
			channel <- m.listAllocations()
		case channel := <-m.listQueueChannel:
			channel <- m.listQueue()
		case topo := <-m.topologyChannel:
			if topo != nil {
				m.topology = topo
				m.params.Logger.Debugln(0, "received topology")
				recalculate = true
			} // nil case is for unittests.
		case entry := <-removeChannel:
			if err := m.params.Storer.DeleteUpdate(entry.Position); err != nil {
				m.params.Logger.Println(err)
			}
			delete(m.deleted, entry.Value.RequestId)
		case update, ok := <-updateChannel:
			if !ok {
				m.params.Logger.Println("update channel closed: remaking")
				updateChannel = m.params.UpdateChannelMaker.MakeUpdateChannel(
					fm_proto.GetUpdatesRequest{IgnoreMissingLocalTags: true})
				resetResources = true
			} else {
				m.processUpdate(update, &resetResources)
				recalculate = true
			}
		case <-expireTimer.C:
			checkExpirations = true
			recalculate = true
		case <-timer.C:
		}
		clearTimer(timer)
		timerInterval := m.params.managerInterval
		if recalculate && m.topology != nil {
			startTime := time.Now()
			requestId := m.recalculate()
			timeTaken := time.Since(startTime)
			if requestId != "" {
				checkExpirations = true
				timerInterval = time.Millisecond
				m.params.Logger.Debugf(0,
					"calculated allocation for: %s in: %s\n",
					requestId, format.Duration(timeTaken))
			}
			recalculateTimeDistribution.Add(timeTaken)
		}
		if checkExpirations {
			clearTimer(expireTimer)
			var err error
			nextExpiration, err = m.expireRequests(false)
			if err != nil {
				m.params.Logger.Println(err)
			}
			expireTimer.Reset(time.Until(nextExpiration))
		}
		timer.Reset(timerInterval)
		select {
		case m.heartbeatChannel <- struct{}{}:
		default:
		}
	}
}

func (m *manager) makeId() fm_proto.RequestId {
	currentIdTime := time.Now().Format("2006-01-02:15:04:05")
	if m.lastIdTime == currentIdTime {
		m.lastSequence++
	} else {
		m.lastIdTime = currentIdTime
		m.lastSequence = 0
	}
	return fm_proto.RequestId(
		fmt.Sprintf("%s-%s-%d", m.myIP, m.lastIdTime, m.lastSequence))
}

func (m *manager) processUpdate(update fm_proto.Update, resetResources *bool) {
	m.params.Logger.Debugln(1, "processUpdate()")
	if *resetResources {
		m.initialiseResources()
		*resetResources = false
	}
	for _, machine := range update.ChangedMachines {
		m.machines[machine.Hostname] = machine
	}
	for _, hostname := range update.DeletedMachines {
		delete(m.machines, hostname)
		delete(m.hypervisorDatas, hostname)
	}
	for ipAddr, vm := range update.ChangedVMs {
		m.vmToHypervisor[ipAddr] = update.VmToHypervisor[ipAddr]
		m.vms[ipAddr] = vm
		m.processVmUpdate(vm, ipAddr, update.VmToHypervisor[ipAddr])
	}
	for _, ipAddr := range update.DeletedVMs {
		delete(m.vmToHypervisor, ipAddr)
		delete(m.vms, ipAddr)
	}
	for hostname, hypervisorData := range update.ChangedHypervisors {
		m.hypervisorDatas[hostname] = hypervisorData
	}
}

func (m *manager) processVmUpdate(vm *hyper_proto.VmInfo, vmIpAddr string,
	hypervisorHostname string) {
	requestId := fm_proto.RequestId(vm.Tags["AllocationRequestId"])
	if requestId == "" {
		return
	}
	allocation := m.allocations[requestId]
	if allocation == nil {
		m.params.Logger.Debugf(1,
			"processVmUpdate(%s): no allocation for request: %s\n",
			vm.Address.IpAddress, requestId)
		return
	}
	if allocation.indexToVmIp == nil {
		allocation.indexToVmIp = make(map[int]string)
	}
	if allocation.vmIpToIndex == nil {
		allocation.vmIpToIndex = make(map[string]int)
	}
	if _, ok := allocation.vmIpToIndex[vmIpAddr]; ok {
		return
	}
	foundVm := -1
	for vmIndex, vmSpec := range allocation.request.VMs {
		if _, ok := allocation.indexToVmIp[vmIndex]; ok {
			continue
		}
		if hypervisorHostname != allocation.vmHypervisors[vmIndex] {
			continue
		}
		if !checkVmMatchesSpec(vm, &vmSpec) {
			continue
		}
		foundVm = vmIndex
		break
	}
	if foundVm < 0 {
		m.params.Logger.Printf(
			"processVmUpdate(): VM: %s does not match request: %s\n",
			vmIpAddr, requestId)
		return
	}
	allocation.indexToVmIp[foundVm] = vmIpAddr
	allocation.vmIpToIndex[vmIpAddr] = foundVm
	if len(allocation.indexToVmIp) < len(allocation.allocation.VMs) {
		return
	}
	// All VMs in allocation have been created: discard the allocation.
	deletedEntry := fm_proto.DeletedAllocation{
		Reason: fm_proto.AllocationRequestCompleted,
	}
	err := m.sendDeleted(requestId, nil, deletedEntry, allocation.username)
	if err != nil {
		m.params.Logger.Println(err)
	} else {
		m.params.Logger.Debugf(0,
			"all VMs created for request: %s, sending delete\n", requestId)
		m.deleted[requestId] = &deletedType{
			allocation: allocation.allocation,
			deleted:    &deletedEntry,
			request:    allocation.request,
			username:   allocation.username,
		}
		delete(m.allocations, requestId)
	}
}

func (m *manager) sendAllocation(requestId fm_proto.RequestId,
	request *fm_proto.AllocateRequest, available *fm_proto.Allocation,
	username types.Username) error {
	position := m.updateQueue.Position()
	m.params.Logger.Debugf(1,
		"sendAllocation(%s), request: %+v, available: %+v\n",
		requestId, request, available)
	return m.sendUpdate(fm_proto.AllocationUpdateEntry{
		Available: available,
		Request:   request,
		RequestId: requestId,
		Username:  username,
	},
		position)
}

func (m *manager) sendDeleted(requestId fm_proto.RequestId,
	request *fm_proto.AllocateRequest, deletedEntry fm_proto.DeletedAllocation,
	username types.Username) error {
	position := m.updateQueue.Position()
	m.params.Logger.Debugf(1,
		"sendDeleted(%s), request: %+v, deleted: %+v\n",
		requestId, request, deletedEntry)
	return m.sendUpdate(fm_proto.AllocationUpdateEntry{
		Deleted:   &deletedEntry,
		Request:   request,
		RequestId: requestId,
		Username:  username,
	},
		position)
}

func (m *manager) sendUpdate(update fm_proto.AllocationUpdateEntry,
	position uint64) error {
	update.Timestamp = time.Now()
	err := m.params.Storer.WriteUpdate(update, position)
	if err != nil {
		return err
	}
	m.updateChannel <- update
	m.updateQueue.Sync() // Ensure queue position is updated now.
	return nil
}
