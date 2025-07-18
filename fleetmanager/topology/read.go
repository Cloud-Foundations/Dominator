package topology

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"

	"github.com/Cloud-Foundations/Dominator/lib/expand"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
)

type commonStateType struct {
	hostnames    map[string]struct{}
	ipAddresses  map[string]struct{}
	macAddresses map[string]struct{}
}

type inheritingState struct {
	installConfig *InstallConfig      // Replace semantics.
	owners        *ownersType         // Merge semantics.
	subnetIds     map[string]struct{} // Merge semantics.
	tags          tags.Tags           // Merge semantics.
}

func checkMacAddressIsZero(macAddr proto.HardwareAddr) bool {
	for _, b := range macAddr {
		if b != 0 {
			return false
		}
	}
	return true
}

func cloneSet(set map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(set))
	for key := range set {
		clone[key] = struct{}{}
	}
	return clone
}

func load(params Params) (*Topology, error) {
	if params.Logger == nil {
		params.Logger = nulllogger.New()
	}
	topology := &Topology{
		logger:          params.Logger,
		machineParents:  make(map[string]*Directory),
		reservedIpAddrs: make(map[string]struct{}),
	}
	commonState := &commonStateType{
		hostnames:    make(map[string]struct{}),
		ipAddresses:  make(map[string]struct{}),
		macAddresses: make(map[string]struct{}),
	}
	params.Logger.Debugf(1, "loading topology from: %s\n", params.TopologyDir)
	directory, err := topology.readDirectory(params.TopologyDir, "",
		newInheritingState(), commonState)
	if err != nil {
		return nil, err
	}
	topology.Root = directory
	topology.hostIpAddresses = commonState.ipAddresses
	if err := topology.readVariables(params.VariablesDir, ""); err != nil {
		return nil, err
	}
	return topology, nil
}

func loadInstallConfig(filename string) (*InstallConfig, error) {
	var installConfig InstallConfig
	if err := json.ReadFromFile(filename, &installConfig); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading: %s: %s", filename, err)
	}
	return &installConfig, nil
}

func loadMachines(filename string, logger log.DebugLogger) (
	[]*proto.Machine, error) {
	var machines, rawMachines []*proto.Machine
	if err := json.ReadFromFile(filename, &machines); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading: %s: %s", filename, err)
	}
	for _, machine := range rawMachines {
		if len(machine.HostIpAddress) == 0 {
			if addrs, err := net.LookupIP(machine.Hostname); err != nil {
				logger.Printf("not including machine: %s\n", err)
				continue
			} else {
				var addrsIPv4 []net.IP
				for _, addr := range addrs {
					if addrIPv4 := addr.To4(); addrIPv4 != nil {
						addrsIPv4 = append(addrsIPv4, addrIPv4)
					}
				}
				if len(addrsIPv4) != 1 {
					return nil, fmt.Errorf("num IPv4 addresses for: %s: %d!=1",
						machine.Hostname, len(addrsIPv4))
				} else {
					machine.HostIpAddress = addrs[0]
				}
			}
		}
		if len(machine.HostIpAddress) == 16 {
			machine.HostIpAddress = machine.HostIpAddress.To4()
		}
		machines = append(machines, machine)
	}
	return machines, nil
}

func loadOwners(filename string) (*ownersType, error) {
	var owners ownersType
	if err := json.ReadFromFile(filename, &owners); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading: %s: %s", filename, err)
	}
	return &owners, nil
}

func loadSubnets(filename string) ([]*Subnet, error) {
	var subnets []*Subnet
	if err := json.ReadFromFile(filename, &subnets); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading: %s: %s", filename, err)
	}
	gatewayIPs := make(map[string]struct{}, len(subnets))
	for _, subnet := range subnets {
		subnet.Shrink()
		gatewayIp := subnet.IpGateway.String()
		if _, ok := gatewayIPs[gatewayIp]; ok {
			return nil, fmt.Errorf("duplicate gateway IP: %s", gatewayIp)
		} else {
			gatewayIPs[gatewayIp] = struct{}{}
		}
		subnet.reservedIpAddrs = make(map[string]struct{})
		for _, ipAddr := range subnet.ReservedIPs {
			subnet.reservedIpAddrs[ipAddr.String()] = struct{}{}
		}
	}
	return subnets, nil
}

func loadTags(filename string) (tags.Tags, error) {
	var loadedTags tags.Tags
	if err := json.ReadFromFile(filename, &loadedTags); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading: %s: %s", filename, err)
	}
	return loadedTags, nil
}

func (cState *commonStateType) addHostname(name string) error {
	if name == "" {
		return nil
	}
	if _, ok := cState.hostnames[name]; ok {
		return fmt.Errorf("duplicate hostname: %s", name)
	}
	cState.hostnames[name] = struct{}{}
	return nil
}

func (cState *commonStateType) addIpAddress(ipAddr net.IP) error {
	if len(ipAddr) < 1 {
		return nil
	}
	name := ipAddr.String()
	if _, ok := cState.ipAddresses[name]; ok {
		return fmt.Errorf("duplicate IP address: %s", name)
	}
	cState.ipAddresses[name] = struct{}{}
	return nil
}

func (cState *commonStateType) addMacAddress(
	macAddr proto.HardwareAddr) error {
	if len(macAddr) < 1 {
		return nil
	}
	if checkMacAddressIsZero(macAddr) {
		return nil
	}
	name := macAddr.String()
	if _, ok := cState.macAddresses[name]; ok {
		return fmt.Errorf("duplicate MAC address: %s", name)
	}
	cState.macAddresses[name] = struct{}{}
	return nil
}

func (cState *commonStateType) addMachine(machine *proto.Machine,
	subnetIds map[string]struct{}) error {
	if machine.GatewaySubnetId != "" {
		if _, ok := subnetIds[machine.GatewaySubnetId]; !ok {
			return fmt.Errorf("unknown gateway subnetId: %s",
				machine.GatewaySubnetId)
		}
	}
	err := cState.addNetworkEntry(machine.NetworkEntry, subnetIds)
	if err != nil {
		return err
	}
	if machine.Hostname == "" {
		machine.Hostname = machine.HostIpAddress.String()
		if err := cState.addHostname(machine.Hostname); err != nil {
			return err
		}
	}
	if err := cState.addNetworkEntry(machine.IPMI, nil); err != nil {
		return err
	}
	for _, entry := range machine.SecondaryNetworkEntries {
		if err := cState.addNetworkEntry(entry, subnetIds); err != nil {
			return err
		}
	}
	return nil
}

func (cState *commonStateType) addNetworkEntry(entry proto.NetworkEntry,
	subnetIds map[string]struct{}) error {
	if entry.SubnetId != "" {
		if _, ok := subnetIds[entry.SubnetId]; !ok {
			return fmt.Errorf("unknown netentry subnetId: %s", entry.SubnetId)
		}
		if entry.Hostname != "" {
			return fmt.Errorf(
				"cannot specify SubnetId(%s) and Hostname(%s) together",
				entry.SubnetId, entry.Hostname)
		}
		if len(entry.HostIpAddress) > 0 {
			return fmt.Errorf(
				"cannot specify SubnetId(%s) and HostIpAddress(%s) together",
				entry.SubnetId, entry.HostIpAddress)
		}
	}
	if err := cState.addHostname(entry.Hostname); err != nil {
		return err
	}
	if err := cState.addIpAddress(entry.HostIpAddress); err != nil {
		return err
	}
	if err := cState.addMacAddress(entry.HostMacAddress); err != nil {
		return err
	}
	return nil
}

func newInheritingState() *inheritingState {
	return &inheritingState{
		owners:    &ownersType{},
		subnetIds: cloneSet(nil),
		tags:      make(tags.Tags),
	}
}

func (iState *inheritingState) copy() *inheritingState {
	return &inheritingState{
		installConfig: iState.installConfig,
		owners:        iState.owners.copy(),
		subnetIds:     cloneSet(iState.subnetIds),
		tags:          iState.tags.Copy(),
	}
}

func (t *Topology) loadSubnets(directory *Directory, dirpath string,
	subnetIds map[string]struct{}) error {
	if err := directory.loadSubnets(dirpath, subnetIds); err != nil {
		return err
	}
	for _, subnet := range directory.Subnets {
		for ipAddr := range subnet.reservedIpAddrs {
			t.reservedIpAddrs[ipAddr] = struct{}{}
		}
	}
	t.logger.Debugf(2, "T.loadSubnets: subnets: %v\n", subnetIds)
	return nil
}

func (t *Topology) readDirectory(topDir, dirname string,
	iState *inheritingState, cState *commonStateType) (*Directory, error) {
	directory := &Directory{
		logger:           t.logger,
		nameToDirectory:  make(map[string]*Directory),
		path:             dirname,
		subnetIdToSubnet: make(map[string]*Subnet),
	}
	dirpath := filepath.Join(topDir, dirname)
	t.logger.Debugf(1, "T.readDirectory(%s)\n", dirpath)
	if err := directory.loadInstallConfig(dirpath, iState); err != nil {
		return nil, err
	}
	if err := directory.loadOwners(dirpath, iState.owners); err != nil {
		return nil, err
	}
	if err := t.loadSubnets(directory, dirpath, iState.subnetIds); err != nil {
		return nil, err
	}
	if err := directory.loadTags(dirpath, iState.tags); err != nil {
		return nil, err
	}
	if err := t.loadMachines(directory, dirpath, cState, iState); err != nil {
		return nil, err
	}
	dirnames, err := fsutil.ReadDirnames(dirpath, false)
	if err != nil {
		return nil, err
	}
	for _, name := range dirnames {
		if name[0] == '.' {
			continue
		}
		path := filepath.Join(dirname, name)
		fi, err := os.Lstat(filepath.Join(topDir, path))
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			continue
		}
		iState := iState.copy()
		subdir, err := t.readDirectory(topDir, path, iState, cState)
		if err != nil {
			return nil, err
		} else {
			subdir.Name = name
			subdir.parent = directory
			directory.Directories = append(directory.Directories, subdir)
			directory.nameToDirectory[name] = subdir
		}
	}
	return directory, nil
}

func (directory *Directory) loadInstallConfig(dirname string,
	iState *inheritingState) error {
	installConfig, err := loadInstallConfig(filepath.Join(dirname,
		"install-config.json"))
	if err != nil {
		return err
	}
	if installConfig != nil {
		iState.installConfig = installConfig
	}
	directory.InstallConfig = iState.installConfig
	return nil
}

func (directory *Directory) loadMachines(dirname string) error {
	var err error
	directory.Machines, err = loadMachines(
		filepath.Join(dirname, "machines.json"),
		directory.logger)
	if err != nil {
		return err
	}
	for _, machine := range directory.Machines {
		machine.Location = directory.path
		mergedOwners := ownersType{
			OwnerGroups: machine.OwnerGroups,
			OwnerUsers:  machine.OwnerUsers,
		}
		mergedOwners.merge(directory.owners)
		machine.OwnerGroups = mergedOwners.OwnerGroups
		machine.OwnerUsers = mergedOwners.OwnerUsers
		if machine.Tags == nil {
			machine.Tags = directory.Tags
		} else if directory.Tags != nil {
			mergedTags := directory.Tags.Copy()
			mergedTags.Merge(machine.Tags)
			machine.Tags = mergedTags
		}
	}
	return nil
}

func (directory *Directory) loadOwners(dirname string,
	parentOwners *ownersType) error {
	owners, err := loadOwners(filepath.Join(dirname, "owners.json"))
	if err != nil {
		return err
	}
	parentOwners.merge(owners)
	directory.owners = parentOwners
	return nil
}

func (directory *Directory) loadSubnets(dirname string,
	subnetIds map[string]struct{}) error {
	var err error
	directory.Subnets, err = loadSubnets(filepath.Join(dirname, "subnets.json"))
	if err != nil {
		return err
	}
	for _, subnet := range directory.Subnets {
		if _, ok := subnetIds[subnet.Id]; ok {
			return fmt.Errorf("duplicate subnet ID: %s", subnet.Id)
		} else {
			subnetIds[subnet.Id] = struct{}{}
			directory.subnetIdToSubnet[subnet.Id] = subnet
		}
	}
	return nil
}

func (directory *Directory) loadTags(dirname string,
	parentTags tags.Tags) error {
	loadedTags, err := loadTags(filepath.Join(dirname, "tags.json"))
	if err != nil {
		return err
	}
	parentTags.Merge(loadedTags)
	if len(parentTags) > 0 {
		directory.Tags = parentTags
	}
	return nil
}

func (owners *ownersType) copy() *ownersType {
	newOwners := ownersType{
		OwnerGroups: make([]string, 0, len(owners.OwnerGroups)),
		OwnerUsers:  make([]string, 0, len(owners.OwnerUsers)),
	}
	for _, group := range owners.OwnerGroups {
		newOwners.OwnerGroups = append(newOwners.OwnerGroups, group)
	}
	for _, user := range owners.OwnerUsers {
		newOwners.OwnerUsers = append(newOwners.OwnerUsers, user)
	}
	return &newOwners
}

func (to *ownersType) merge(from *ownersType) {
	if from == nil {
		return
	}
	ownerGroups := stringutil.ConvertListToMap(to.OwnerGroups, false)
	changedOwnerGroups := false
	for _, group := range from.OwnerGroups {
		if _, ok := ownerGroups[group]; !ok {
			to.OwnerGroups = append(to.OwnerGroups, group)
			changedOwnerGroups = true
		}
	}
	if changedOwnerGroups {
		sort.Strings(to.OwnerGroups)
	}
	ownerUsers := stringutil.ConvertListToMap(to.OwnerUsers, false)
	changedOwnerUsers := false
	for _, group := range from.OwnerUsers {
		if _, ok := ownerUsers[group]; !ok {
			to.OwnerUsers = append(to.OwnerUsers, group)
			changedOwnerUsers = true
		}
	}
	if changedOwnerUsers {
		sort.Strings(to.OwnerUsers)
	}
}

func (t *Topology) loadMachines(directory *Directory, dirname string,
	cState *commonStateType, iState *inheritingState) error {
	if err := directory.loadMachines(dirname); err != nil {
		return err
	}
	for _, machine := range directory.Machines {
		err := cState.addMachine(machine, iState.subnetIds)
		if err != nil {
			return fmt.Errorf("error adding: %s: %s", machine.Hostname, err)
		}
		t.machineParents[machine.Hostname] = directory
	}
	return nil
}

func (t *Topology) readVariables(topDir, dirname string) error {
	localVariables := make(map[string]string)
	filename := filepath.Join(topDir, dirname, "local_variables.json")
	if err := json.ReadFromFile(filename, &localVariables); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	variables := make(map[string]string)
	filename = filepath.Join(topDir, dirname, "variables.json")
	if err := json.ReadFromFile(filename, &variables); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	if len(variables) > 0 && t.Variables == nil {
		t.Variables = make(map[string]string)
	}
	for key, value := range variables {
		if len(localVariables) > 0 {
			value = expand.Expression(value, func(variable string) string {
				return localVariables[variable]
			})
		}
		t.Variables[filepath.Join(dirname, key)] = value
	}
	dirpath := filepath.Join(topDir, dirname)
	dirnames, err := fsutil.ReadDirnames(dirpath, true)
	if err != nil {
		return err
	}
	for _, name := range dirnames {
		if name[0] == '.' {
			continue
		}
		path := filepath.Join(dirname, name)
		fi, err := os.Lstat(filepath.Join(topDir, path))
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			continue
		}
		if err := t.readVariables(topDir, path); err != nil {
			return err
		}
	}
	return nil
}
