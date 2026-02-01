package hypervisors

import (
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/fleetmanager/topology"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/firmware"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type addressPoolOptionsType struct {
	desiredSize uint
	maximumSize uint
	minimumSize uint
}

var (
	defaultAddressPoolOptions addressPoolOptionsType
	errorNoAccessToResource   = errors.New("no access to resource")
	manageHypervisors         = flag.Bool("manageHypervisors", false,
		"If true, manage hypervisors")
)

func init() {
	flag.UintVar(&defaultAddressPoolOptions.desiredSize,
		"desiredAddressPoolSize", 16,
		"Desired number of free addresses to maintain in Hypervisor")
	flag.UintVar(&defaultAddressPoolOptions.maximumSize,
		"maximumAddressPoolSize", 24,
		"Maximum number of free addresses to maintain in Hypervisor")
	flag.UintVar(&defaultAddressPoolOptions.minimumSize,
		"minimumAddressPoolSize", 8,
		"Minimum number of free addresses to maintain in Hypervisor")
}

func checkPoolLimits() error {
	if defaultAddressPoolOptions.desiredSize <
		defaultAddressPoolOptions.minimumSize {
		return fmt.Errorf(
			"desiredAddressPoolSize: %d is less than minimumAddressPoolSize: %d",
			defaultAddressPoolOptions.desiredSize,
			defaultAddressPoolOptions.minimumSize)
	}
	if defaultAddressPoolOptions.desiredSize >
		defaultAddressPoolOptions.maximumSize {
		return fmt.Errorf(
			"desiredAddressPoolSize: %d is greater than maximumAddressPoolSize: %d",
			defaultAddressPoolOptions.desiredSize,
			defaultAddressPoolOptions.maximumSize)
	}
	return nil
}

func testInLocation(location, enclosingLocation string) bool {
	if enclosingLocation != "" && location != enclosingLocation {
		if len(enclosingLocation) >= len(location) {
			return false
		}
		if location[len(enclosingLocation)] != '/' {
			return false
		}
		if location[:len(enclosingLocation)] != enclosingLocation {
			return false
		}
	}
	return true
}

func (h *hypervisorType) address() string {
	hostname := h.Machine.Hostname
	if len(h.Machine.HostIpAddress) > 0 {
		hostname = h.Machine.HostIpAddress.String()
	}
	return fmt.Sprintf("%s:%d", hostname, constants.HypervisorPortNumber)
}

func (h *hypervisorType) changeOwners(client *srpc.Client) error {
	if !*manageHypervisors {
		return nil
	}
	if client == nil {
		var err error
		client, err = srpc.DialHTTP("tcp", h.address(), time.Second*15)
		if err != nil {
			return err
		}
		defer client.Close()
	}
	request := hyper_proto.ChangeOwnersRequest{
		OwnerGroups: h.Machine.OwnerGroups,
		OwnerUsers:  h.Machine.OwnerUsers,
	}
	var reply hyper_proto.ChangeOwnersResponse
	err := client.RequestReply("Hypervisor.ChangeOwners", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func (h *hypervisorType) checkAuth(authInfo *srpc.AuthInformation) error {
	if authInfo.HaveMethodAccess {
		return nil
	}
	if _, ok := h.ownerUsers[authInfo.Username]; ok {
		return nil
	}
	for _, ownerGroup := range h.Machine.OwnerGroups {
		if _, ok := authInfo.GroupList[ownerGroup]; ok {
			return nil
		}
	}
	return errorNoAccessToResource
}

func (h *hypervisorType) getMachineLocked() *fm_proto.Machine {
	machine := h.Machine
	if len(h.localTags) < 1 {
		return &machine
	}
	machine.Tags = h.Machine.Tags.Copy()
	machine.Tags.Merge(h.localTags)
	return &machine
}

func (m *Manager) changeMachineTags(hostname string,
	authInfo *srpc.AuthInformation, tgs tags.Tags) error {
	if !*manageHypervisors {
		return errors.New("this is a read-only Fleet Manager")
	}
	if h, err := m.getLockedHypervisor(hostname, true); err != nil {
		return err
	} else if err := h.checkAuth(authInfo); err != nil {
		h.mutex.Unlock()
		return err
	} else {
		for key, localVal := range tgs { // Delete duplicates.
			if machineVal := h.Machine.Tags[key]; localVal == machineVal {
				delete(tgs, key)
			}
		}
		err := m.storer.WriteMachineTags(h.Machine.HostIpAddress, tgs)
		if err != nil {
			h.mutex.Unlock()
			return err
		}
		if len(tgs) > 0 {
			h.localTags = tgs
		} else {
			h.localTags = nil
		}
		update := &fm_proto.Update{
			ChangedMachines: []*fm_proto.Machine{h.getMachineLocked()},
		}
		location := h.location
		h.mutex.Unlock()
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.sendUpdate(location, update)
		return nil
	}
}

func (h *hypervisorType) getMachine() *fm_proto.Machine {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.getMachineLocked()
}

func (m *Manager) closeUpdateChannel(channel <-chan fm_proto.Update) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if location, ok := m.notifiers[channel]; ok {
		delete(location.notifiers, channel)
		delete(m.notifiers, channel)
	}
}

func (m *Manager) makeUpdateChannel(
	request fm_proto.GetUpdatesRequest) <-chan fm_proto.Update {
	// Wait for topology loaded.
	m.topologyLoaded <- struct{}{}
	<-m.topologyLoaded
	channel := make(chan fm_proto.Update, 16)
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !*manageHypervisors && !request.IgnoreMissingLocalTags {
		channel <- fm_proto.Update{Error: "this is a read-only Fleet Manager"}
		return channel
	}
	if m.locations == nil {
		m.locations = make(map[string]*locationType)
	}
	if m.notifiers == nil {
		m.notifiers = make(map[<-chan fm_proto.Update]*locationType)
	}
	location, ok := m.locations[request.Location]
	if !ok {
		location = &locationType{
			notifiers: make(map[<-chan fm_proto.Update]chan<- fm_proto.Update),
		}
		m.locations[request.Location] = location
	}
	location.notifiers[channel] = channel
	m.notifiers[channel] = location
	machines := make([]*fm_proto.Machine, 0)
	hypervisorDatas := make(map[string]fm_proto.HypervisorData)
	vms := make(map[string]*hyper_proto.VmInfo, len(m.vms))
	vmToHypervisor := make(map[string]string, len(m.vms))
	for _, h := range m.hypervisors {
		if !testInLocation(h.location, request.Location) {
			continue
		}
		machines = append(machines, h.getMachine())
		hypervisorDatas[h.Hostname] = h.HypervisorData
		for addr, vm := range h.vms {
			vms[addr] = &vm.VmInfo
			vmToHypervisor[addr] = h.Machine.Hostname
		}
	}
	m.logger.Debugf(0, "makeUpdateChannel(): sending %d machines and %d VMs\n",
		len(machines), len(vms))
	channel <- fm_proto.Update{
		ChangedHypervisors: hypervisorDatas,
		ChangedMachines:    machines,
		ChangedVMs:         vms,
		VmToHypervisor:     vmToHypervisor,
	}
	return channel
}

func (m *Manager) updateHypervisor(h *hypervisorType, machine fm_proto.Machine,
	machineChanged bool) {
	var numTagsToDelete uint
	h.mutex.Lock()
	locationChanged := h.location != machine.Location
	h.location = machine.Location
	machine.MemoryInMiB = h.Machine.MemoryInMiB
	machine.NumCPUs = h.Machine.NumCPUs
	machine.TotalVolumeBytes = h.Machine.TotalVolumeBytes
	h.Machine = machine
	h.ownerUsers = stringutil.ConvertListToMap(machine.OwnerUsers, false)
	subnets := h.subnets
	for key, localVal := range h.localTags {
		if machineVal, ok := h.Machine.Tags[key]; ok && localVal == machineVal {
			delete(h.localTags, key)
			numTagsToDelete++
		}
	}
	if numTagsToDelete > 0 {
		err := m.storer.WriteMachineTags(h.Machine.HostIpAddress, h.localTags)
		if err != nil {
			h.logger.Printf("error writing tags: %s\n", err)
		} else {
			h.logger.Debugf(0, "Deleted %d obsolete local tags\n",
				numTagsToDelete)
		}
	}
	if locationChanged {
		for _, vm := range h.vms {
			vm.Location = h.location
		}
	}
	h.mutex.Unlock()
	if *manageHypervisors && h.probeStatus == probeStatusConnected {
		if machineChanged {
			go h.changeOwners(nil)
		}
		go m.processSubnetsUpdates(h, subnets)
	}
}

func (m *Manager) updateTopology(t *topology.Topology) {
	machines, err := t.ListMachines("")
	if err != nil {
		m.logger.Println(err)
		return
	}
	var waitGroup sync.WaitGroup
	deleteList := m.updateTopologyLocked(t, machines, &waitGroup)
	for _, hypervisor := range deleteList {
		m.storer.UnregisterHypervisor(hypervisor.Machine.HostIpAddress)
		hypervisor.delete()
	}
	waitGroup.Wait()
	// Signal topology loaded.
	select {
	case <-m.topologyLoaded:
	default:
	}
}

func (m *Manager) updateTopologyLocked(t *topology.Topology,
	machines []*fm_proto.Machine, waitGroup *sync.WaitGroup) []*hypervisorType {
	hypervisorsToDelete := make(map[string]struct{}, len(machines))
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.topology = t
	for hypervisorName := range m.hypervisors {
		hypervisorsToDelete[hypervisorName] = struct{}{}
	}
	var hypersToChange, hypersToDelete []*hypervisorType
	for _, machine := range machines {
		delete(hypervisorsToDelete, machine.Hostname)
		if hypervisor, ok := m.hypervisors[machine.Hostname]; ok {
			equal := hypervisor.Machine.Equal(machine)
			if !equal {
				hypersToChange = append(hypersToChange, hypervisor)
			}
			m.updateHypervisor(hypervisor, *machine, !equal)
		} else {
			location, _ := m.topology.GetLocationOfMachine(machine.Hostname)
			hypervisor := &hypervisorType{
				logger:       prefixlogger.New(machine.Hostname+": ", m.logger),
				location:     location,
				Hypervisor:   fm_proto.Hypervisor{Machine: *machine},
				migratingVms: make(map[string]*vmInfoType),
				ownerUsers: stringutil.ConvertListToMap(machine.OwnerUsers,
					false),
				vms: make(map[string]*vmInfoType),
			}
			m.hypervisors[machine.Hostname] = hypervisor
			if hostMac := machine.HostMacAddress.String(); hostMac != "" {
				m.hypervisorsByHW[hostMac] = hypervisor
			}
			m.hypervisorsByIP[machine.HostIpAddress.String()] = hypervisor
			hypersToChange = append(hypersToChange, hypervisor)
			waitGroup.Add(1)
			go m.manageHypervisorLoop(hypervisor, waitGroup)
		}
	}
	deleteList := make([]*hypervisorType, 0, len(hypervisorsToDelete))
	for hypervisorName := range hypervisorsToDelete {
		hypervisor := m.hypervisors[hypervisorName]
		deleteList = append(deleteList, hypervisor)
		delete(m.hypervisors, hypervisorName)
		delete(m.hypervisorsByHW, hypervisor.HostMacAddress.String())
		delete(m.hypervisorsByIP, hypervisor.HostIpAddress.String())
		if hypervisor.serialNumber != "" &&
			m.hypervisorsBySN[hypervisor.serialNumber] == hypervisor {
			delete(m.hypervisorsBySN, hypervisor.serialNumber)
		}
		hypersToDelete = append(hypersToDelete, hypervisor)
		for vmIP := range hypervisor.migratingVms {
			delete(m.vms, vmIP)
		}
		for vmIP := range hypervisor.vms {
			delete(m.vms, vmIP)
		}
	}
	if len(hypersToChange) > 0 || len(hypersToDelete) > 0 {
		updates := m.splitChanges(hypersToChange, hypersToDelete)
		for location, updateForLocation := range updates {
			m.sendUpdate(location, updateForLocation)
		}
	}
	// (Re)build map of managed subnets.
	m.subnets = make(map[string]*subnetType, len(m.subnets))
	t.Walk(func(directory *topology.Directory) error {
		for _, tSubnet := range directory.Subnets {
			if !tSubnet.Manage {
				continue
			}
			gatewayIp := tSubnet.IpGateway.String()
			if _, ok := m.subnets[gatewayIp]; ok {
				m.logger.Printf(
					"ignoring duplicate subnet Id: %s gateway IP: %s\n",
					tSubnet.Id, gatewayIp)
				continue
			}
			m.subnets[gatewayIp] = m.makeSubnet(tSubnet)
		}
		return nil
	})
	return deleteList
}

func (h *hypervisorType) delete() {
	h.logger.Debugln(0, "deleting")
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.deleteScheduled = true
	select {
	case h.closeClientChannel <- struct{}{}:
	default:
	}
}

func (h *hypervisorType) isDeleteScheduled() bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.deleteScheduled
}

// manageHypervisorLoop loads information about the Hypervisor and then manages
// it until it is scheduled for deletion. The WaitGroup is decremented after
// loading completes.
func (m *Manager) manageHypervisorLoop(h *hypervisorType, wg *sync.WaitGroup) {
	defer func() { // Ensure the WaitGroup is decremented.
		if wg != nil {
			wg.Done()
		}
	}()
	vmList, err := m.storer.ListVMs(h.Machine.HostIpAddress)
	if err != nil {
		h.logger.Printf("error reading VMs, not managing hypervisor: %s", err)
		return
	}
	h.cachedSerialNumber, err = m.storer.ReadMachineSerialNumber(
		h.Machine.HostIpAddress)
	if err != nil {
		h.logger.Printf(
			"error reading serial number, not managing hypervisor: %s", err)
		return
	}
	h.serialNumber = h.cachedSerialNumber
	m.probeSerialNumber(h)
	h.localTags, err = m.storer.ReadMachineTags(h.Machine.HostIpAddress)
	if err != nil {
		h.logger.Printf("error reading tags, not managing hypervisor: %s", err)
		return
	}
	for _, vmIpAddr := range vmList {
		pVmInfo, err := m.storer.ReadVm(h.Machine.HostIpAddress, vmIpAddr)
		if err != nil {
			h.logger.Printf("error reading VM: %s: %s", vmIpAddr, err)
			continue
		}
		vmInfo := &vmInfoType{vmIpAddr, *pVmInfo, h.location, h,
			h.Machine.Hostname}
		h.vms[vmIpAddr] = vmInfo
		m.mutex.Lock()
		m.vms[vmIpAddr] = vmInfo
		m.mutex.Unlock()
	}
	wg.Done() // Loading completed: notify.
	wg = nil
	for !h.isDeleteScheduled() {
		sleepTime := m.manageHypervisor(h)
		time.Sleep(sleepTime)
	}
}

func (m *Manager) manageHypervisor(h *hypervisorType) time.Duration {
	failureProbeStatus := probeStatusUnreachable
	defer func() {
		h.mutex.Lock()
		defer h.mutex.Unlock()
		h.closeClientChannel = nil
		h.probeStatus = failureProbeStatus
	}()
	client, err := srpc.DialHTTP("tcp", h.address(), time.Second*15)
	if err != nil {
		h.logger.Debugln(1, err)
		switch err {
		case srpc.ErrorAccessToMethodDenied:
			failureProbeStatus = probeStatusAccessDenied
		case srpc.ErrorNoSrpcEndpoint:
			failureProbeStatus = probeStatusNoSrpc
		case srpc.ErrorConnectionRefused:
			failureProbeStatus = probeStatusConnectionRefused
		default:
			failureProbeStatus = m.probeUnreachable(h)
		}
		return time.Second
	}
	defer client.Close()
	if err := h.changeOwners(client); err != nil {
		if strings.HasPrefix(err.Error(), "unknown service") {
			h.logger.Debugln(1, err)
		} else {
			h.logger.Println(err)
		}
	}
	conn, err := client.Call("Hypervisor.GetUpdates")
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown service") {
			h.logger.Debugln(1, err)
			failureProbeStatus = probeStatusNoService
			return time.Minute
		} else {
			h.logger.Println(err)
		}
		return time.Second
	}
	h.mutex.Lock()
	h.probeStatus = probeStatusConnected
	if h.deleteScheduled {
		h.mutex.Unlock()
		conn.Close()
		return 0
	}
	closeClientChannel := make(chan struct{}, 1)
	h.closeClientChannel = closeClientChannel
	h.receiveChannel = make(chan struct{}, 1)
	h.mutex.Unlock()
	go h.monitorLoop(client, conn, closeClientChannel)
	defer close(h.receiveChannel)
	h.logger.Debugln(0, "waiting for Update messages")
	firstUpdate := true
	for {
		var update hyper_proto.Update
		if err := conn.Decode(&update); err != nil {
			if err == io.EOF {
				h.logger.Debugln(0, "remote closed connection")
			} else if !client.IsClosed() {
				h.logger.Println(err)
			}
			return time.Second
		}
		h.receiveChannel <- struct{}{}
		m.processHypervisorUpdate(h, update, firstUpdate)
		firstUpdate = false
	}
}

func (m *Manager) getSubnetsForMachine(h *hypervisorType) (
	map[string]*topology.Subnet, error) {
	m.mutex.Lock()
	subnetsSlice, err := m.topology.GetSubnetsForMachine(h.Machine.Hostname)
	m.mutex.Unlock()
	if err != nil {
		return nil, err
	}
	subnetsMap := make(map[string]*topology.Subnet, len(subnetsSlice))
	for _, subnet := range subnetsSlice {
		subnetsMap[subnet.Id] = subnet
	}
	return subnetsMap, nil
}

func (m *Manager) processAddressPoolUpdates(h *hypervisorType,
	update hyper_proto.Update) {
	// First record what IPs the Hypervisor currently has.
	if update.HaveAddressPool {
		h.logger.Debugf(1, "registered address pool size: %d\n",
			len(update.AddressPool))
		addresses := make([]net.IP, 0, len(update.AddressPool))
		for _, address := range update.AddressPool {
			addresses = append(addresses, address.IpAddress)
		}
		err := m.storer.SetIPsForHypervisor(h.Machine.HostIpAddress,
			addresses)
		if err != nil {
			h.logger.Println(err)
		}
	}
	if !*manageHypervisors {
		return
	}
	// Compute adjustments needed to Hypervisor address pool.
	ipsToAdd := make([]net.IP, 0)
	addressesToAdd := make([]hyper_proto.Address, 0)
	maxFreeAddresses := make(map[string]uint)
	tSubnets, err := m.getSubnetsForMachine(h)
	if err != nil {
		h.logger.Println(err)
		return
	}
	addressPoolOptions := defaultAddressPoolOptions
	if h.disabled {
		addressPoolOptions.desiredSize = 0
		addressPoolOptions.maximumSize = 0
		addressPoolOptions.minimumSize = 0
	} else if h.healthStatus == "marginal" || h.healthStatus == "at risk" {
		addressPoolOptions.desiredSize = 1
		addressPoolOptions.maximumSize = 1
		addressPoolOptions.minimumSize = 1
	}
	var numAddressesToRemove uint
	for subnetId, numFreeAddresses := range update.NumFreeAddresses {
		tSubnet := tSubnets[subnetId]
		if tSubnet == nil {
			h.logger.Printf("update for missing subnet: %s\n", subnetId)
			return
		}
		if !tSubnet.Manage {
			continue
		}
		if numFreeAddresses < addressPoolOptions.minimumSize {
			m.mutex.Lock()
			freeIPs, err := m.findFreeIPs(tSubnet,
				addressPoolOptions.desiredSize-numFreeAddresses)
			defer m.unmarkAllocatingIPs(freeIPs)
			m.mutex.Unlock()
			if err != nil {
				h.logger.Println(err)
				return
			}
			if len(freeIPs) < 1 {
				continue
			}
			for _, ip := range freeIPs {
				ipsToAdd = append(ipsToAdd, ip)
				addressesToAdd = append(addressesToAdd, hyper_proto.Address{
					IpAddress: ip,
					MacAddress: fmt.Sprintf("52:54:%02x:%02x:%02x:%02x",
						ip[0], ip[1], ip[2], ip[3]),
				})
			}
			h.logger.Debugf(0, "Adding %d addresses to subnet: %s\n",
				len(freeIPs), subnetId)
		} else if numFreeAddresses > addressPoolOptions.maximumSize {
			maxFreeAddresses[subnetId] = addressPoolOptions.desiredSize
			numAddressesToRemove += numFreeAddresses -
				addressPoolOptions.desiredSize
			h.logger.Debugf(0, "Removing %d excess addresses from subnet: %s\n",
				numFreeAddresses-addressPoolOptions.maximumSize, subnetId)
		}
	}
	if len(addressesToAdd) < 1 && len(maxFreeAddresses) < 1 {
		return
	}
	// Send change command to Hypervisor.
	client, err := srpc.DialHTTP("tcp", h.address(), time.Minute)
	if err != nil {
		h.logger.Println(err)
		return
	}
	defer client.Close()
	request := hyper_proto.ChangeAddressPoolRequest{
		AddressesToAdd:       addressesToAdd,
		MaximumFreeAddresses: maxFreeAddresses,
	}
	var reply hyper_proto.ChangeAddressPoolResponse
	err = client.RequestReply("Hypervisor.ChangeAddressPool",
		request, &reply)
	if err == nil {
		err = errors.New(reply.Error)
	}
	if err != nil {
		h.logger.Println(err)
		return
	}
	// Record changes made. If the Hypervisor can't make the change, that will
	// be recorded in the next update.
	m.storer.AddIPsForHypervisor(h.Machine.HostIpAddress, ipsToAdd)
	if len(addressesToAdd) > 0 {
		h.logger.Debugf(0, "replenished pool with %d addresses\n",
			len(addressesToAdd))
	}
	if len(maxFreeAddresses) > 0 {
		h.logger.Debugf(0, "removed %d excess addresses from pool\n",
			numAddressesToRemove)
	}
}

func (m *Manager) processHypervisorUpdate(h *hypervisorType,
	update hyper_proto.Update, firstUpdate bool) {
	h.logger.Debugln(1, "processHypervisorUpdate()")
	h.mutex.Lock()
	oldData := h.HypervisorData
	oldMachine := h.Machine
	if update.AvailableMemoryInMiB != nil {
		h.AvailableMemory = *update.AvailableMemoryInMiB
	}
	if update.HaveDisabled {
		h.disabled = update.Disabled
	}
	if update.MemoryInMiB != nil {
		h.MemoryInMiB = *update.MemoryInMiB
	}
	if update.NumCPUs != nil {
		h.NumCPUs = *update.NumCPUs
	}
	if update.NumFreeAddresses != nil {
		h.NumFreeAddresses = update.NumFreeAddresses
	}
	if update.TotalVolumeBytes != nil {
		h.TotalVolumeBytes = *update.TotalVolumeBytes
	}
	oldHealthStatus := h.healthStatus
	h.healthStatus = update.HealthStatus
	oldSerialNumber := h.serialNumber
	update.SerialNumber = firmware.ExtractSerialNumber(update.SerialNumber)
	if update.HaveSerialNumber && update.SerialNumber != "" {
		h.serialNumber = update.SerialNumber
	}
	h.mutex.Unlock()
	m.updateSerialNumberMap(h, update.SerialNumber)
	if !firstUpdate && update.HealthStatus != oldHealthStatus {
		h.logger.Printf("health status changed from: \"%s\" to: \"%s\"\n",
			oldHealthStatus, update.HealthStatus)
	}
	if *manageHypervisors {
		if update.HaveSubnets { // Must do subnets first.
			h.mutex.Lock()
			h.subnets = update.Subnets
			h.mutex.Unlock()
			m.processSubnetsUpdates(h, update.Subnets)
		}
	}
	m.processAddressPoolUpdates(h, update)
	if update.HaveSerialNumber && update.SerialNumber != "" &&
		update.SerialNumber != oldSerialNumber {
		err := m.storer.WriteMachineSerialNumber(h.Machine.HostIpAddress,
			update.SerialNumber)
		if err != nil {
			h.logger.Println(err)
		} else {
			h.mutex.Lock()
			h.cachedSerialNumber = update.SerialNumber
			h.mutex.Unlock()
		}
	}
	var updateToSend fm_proto.Update
	if update.HaveVMs {
		if firstUpdate {
			m.processInitialVMs(h, update.VMs, &updateToSend)
		} else {
			m.processVmUpdates(h, update.VMs, &updateToSend)
		}
	}
	if !h.Machine.Equal(&oldMachine) {
		updateToSend.ChangedMachines = []*fm_proto.Machine{&h.Machine}
	}
	if !h.HypervisorData.Equal(&oldData) {
		updateToSend.ChangedHypervisors = map[string]fm_proto.HypervisorData{
			h.Hostname: h.HypervisorData}
	}
	m.sendUpdate(h.location, &updateToSend)
}

func (m *Manager) processInitialVMs(h *hypervisorType,
	vms map[string]*hyper_proto.VmInfo, updateToSend *fm_proto.Update) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for ipAddr := range h.vms {
		if _, ok := vms[ipAddr]; !ok {
			vms[ipAddr] = nil
		}
	}
	for ipAddr := range h.migratingVms {
		if _, ok := vms[ipAddr]; !ok {
			vms[ipAddr] = nil
		}
	}
	m.processVmUpdatesWithLock(h, vms, updateToSend)
}

func (m *Manager) processSubnetsUpdates(h *hypervisorType,
	haveSubnets []hyper_proto.Subnet) {
	haveSubnetsMap := make(map[string]int, len(haveSubnets))
	for index, subnet := range haveSubnets {
		haveSubnetsMap[subnet.Id] = index
	}
	t, err := m.getTopology()
	if err != nil {
		h.logger.Println(err)
		return
	}
	needSubnets, err := t.GetSubnetsForMachine(h.Machine.Hostname)
	if err != nil {
		h.logger.Println(err)
		return
	}
	subnetsToDelete := make(map[string]struct{}, len(haveSubnets))
	for _, subnet := range haveSubnets {
		subnetsToDelete[subnet.Id] = struct{}{}
	}
	var request hyper_proto.UpdateSubnetsRequest
	for _, needSubnet := range needSubnets {
		if index, ok := haveSubnetsMap[needSubnet.Id]; ok {
			haveSubnet := haveSubnets[index]
			delete(subnetsToDelete, haveSubnet.Id)
			if !needSubnet.Equal(&haveSubnet) {
				request.Change = append(request.Change, needSubnet.Subnet)
			}
		} else {
			request.Add = append(request.Add, needSubnet.Subnet)
		}
	}
	for subnetId := range subnetsToDelete {
		request.Delete = append(request.Delete, subnetId)
	}
	if len(request.Add) < 1 && len(request.Change) < 1 &&
		len(request.Delete) < 1 {
		return
	}
	client, err := srpc.DialHTTP("tcp", h.address(), time.Minute)
	if err != nil {
		h.logger.Println(err)
		return
	}
	defer client.Close()
	var reply hyper_proto.UpdateSubnetsResponse
	err = client.RequestReply("Hypervisor.UpdateSubnets", request, &reply)
	if err == nil {
		err = errors.New(reply.Error)
	}
	if err != nil {
		h.logger.Println(err)
		return
	}
	h.logger.Debugf(0, "Added %d, changed %d and deleted %d subnets\n",
		len(request.Add), len(request.Change), len(request.Delete))
}

func (m *Manager) processVmUpdates(h *hypervisorType,
	updateVMs map[string]*hyper_proto.VmInfo, updateToSend *fm_proto.Update) {
	for ipAddr, vm := range updateVMs {
		if len(vm.Volumes) < 1 {
			updateVMs[ipAddr] = nil
		}
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.processVmUpdatesWithLock(h, updateVMs, updateToSend)
}

func (m *Manager) processVmUpdatesWithLock(h *hypervisorType,
	updateVMs map[string]*hyper_proto.VmInfo, updateToSend *fm_proto.Update) {
	updateToSend.ChangedVMs = make(map[string]*hyper_proto.VmInfo)
	updateToSend.VmToHypervisor = make(map[string]string)
	vmsToDelete := make(map[string]struct{})
	for ipAddr, protoVm := range updateVMs {
		if protoVm == nil {
			if _, ok := h.migratingVms[ipAddr]; !ok {
				vmsToDelete[ipAddr] = struct{}{}
			} else {
				delete(h.migratingVms, ipAddr)
				delete(m.migratingIPs, ipAddr)
				h.logger.Debugf(0, "forgot migrating VM: %s\n", ipAddr)
			}
		} else {
			if protoVm.State == hyper_proto.StateMigrating {
				if _, ok := h.vms[ipAddr]; ok {
					vmsToDelete[ipAddr] = struct{}{}
				}
				h.migratingVms[ipAddr] = &vmInfoType{ipAddr, *protoVm,
					h.location, h, h.Machine.Hostname}
				m.migratingIPs[ipAddr] = struct{}{}
			} else if vm, ok := h.vms[ipAddr]; ok {
				if !vm.VmInfo.Equal(protoVm) {
					err := m.storer.WriteVm(h.Machine.HostIpAddress, ipAddr,
						*protoVm)
					if err != nil {
						h.logger.Printf("error writing VM: %s: %s\n",
							ipAddr, err)
					} else {
						h.logger.Debugf(0, "updated VM: %s\n", ipAddr)
					}
				}
				vm.VmInfo = *protoVm
				updateToSend.ChangedVMs[ipAddr] = protoVm
				updateToSend.VmToHypervisor[ipAddr] = h.Machine.Hostname
			} else {
				if _, ok := h.migratingVms[ipAddr]; ok {
					delete(h.migratingVms, ipAddr)
					delete(m.migratingIPs, ipAddr)
				}
				vm := &vmInfoType{ipAddr, *protoVm, h.location, h,
					h.Machine.Hostname}
				h.vms[ipAddr] = vm
				m.vms[ipAddr] = vm
				err := m.storer.WriteVm(h.Machine.HostIpAddress, ipAddr,
					*protoVm)
				if err != nil {
					h.logger.Printf("error writing VM: %s: %s\n", ipAddr, err)
				} else {
					h.logger.Debugf(0, "wrote VM: %s\n", ipAddr)
				}
				updateToSend.ChangedVMs[ipAddr] = protoVm
				updateToSend.VmToHypervisor[ipAddr] = h.Machine.Hostname
			}
		}
	}
	for ipAddr := range vmsToDelete {
		delete(h.vms, ipAddr)
		delete(m.vms, ipAddr)
		err := m.storer.DeleteVm(h.Machine.HostIpAddress, ipAddr)
		if err != nil {
			h.logger.Printf("error deleting VM: %s: %s\n", ipAddr, err)
		} else {
			h.logger.Debugf(0, "deleted VM: %s\n", ipAddr)
		}
		updateToSend.DeletedVMs = append(updateToSend.DeletedVMs, ipAddr)
	}
	h.AllocatedMilliCPUs = 0
	h.AllocatedMemory = 0
	h.AllocatedVolumeBytes = 0
	for _, vm := range h.vms {
		h.AllocatedMilliCPUs += uint64(vm.MilliCPUs)
		h.AllocatedMemory += vm.MemoryInMiB
		h.AllocatedVolumeBytes += vm.TotalStorage()
	}
}

func (m *Manager) splitChanges(hypersToChange []*hypervisorType,
	hypersToDelete []*hypervisorType) map[string]*fm_proto.Update {
	updates := make(map[string]*fm_proto.Update)
	for _, h := range hypersToChange {
		if locationUpdate, ok := updates[h.location]; !ok {
			updates[h.location] = &fm_proto.Update{
				ChangedMachines: []*fm_proto.Machine{h.getMachine()},
			}
		} else {
			locationUpdate.ChangedMachines = append(
				locationUpdate.ChangedMachines, h.getMachine())
		}
	}
	for _, h := range hypersToDelete {
		if locationUpdate, ok := updates[h.location]; !ok {
			updates[h.location] = &fm_proto.Update{
				DeletedMachines: []string{h.Machine.Hostname},
			}
		} else {
			locationUpdate.DeletedMachines = append(
				locationUpdate.DeletedMachines, h.Machine.Hostname)
		}
	}
	return updates
}

func (m *Manager) sendUpdate(hyperLocation string, update *fm_proto.Update) {
	if len(update.ChangedHypervisors) < 1 &&
		len(update.ChangedMachines) < 1 &&
		len(update.ChangedVMs) < 1 &&
		len(update.DeletedMachines) < 1 &&
		len(update.DeletedVMs) < 1 &&
		len(update.VmToHypervisor) < 1 {
		return
	}
	for locationStr, location := range m.locations {
		if !testInLocation(hyperLocation, locationStr) {
			continue
		}
		for rChannel, sChannel := range location.notifiers {
			select {
			case sChannel <- *update:
			default:
				delete(location.notifiers, rChannel)
				delete(m.notifiers, rChannel)
				close(sChannel)
			}
		}
	}
}

// updateSerialNumberMap will update the serial number map. The Manager lock
// will be grabbed and released.
func (m *Manager) updateSerialNumberMap(h *hypervisorType,
	serialNumber string) {
	if serialNumber == "" {
		return
	}
	m.mutex.Lock()
	oldHypervisor, ok := m.hypervisorsBySN[serialNumber]
	if !ok {
		m.hypervisorsBySN[serialNumber] = h
		oldHypervisor = h
	} else if h != oldHypervisor {
		m.hypervisorsBySN[serialNumber] = nil // Mark duplicate.
	}
	m.mutex.Unlock()
	if h != oldHypervisor {
		if oldHypervisor == nil {
			m.logger.Printf("duplicate serial number: %s found for: %s\n",
				serialNumber, h.Hostname)
		} else {
			m.logger.Printf(
				"duplicate serial number: %s found for: %s and %s\n",
				serialNumber, h.Hostname, oldHypervisor.Hostname)
		}
	}
}
