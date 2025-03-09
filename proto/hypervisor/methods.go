package hypervisor

import (
	"bytes"
	"errors"
	"net"
	"strings"
)

const (
	consoleTypeUnknown     = "UNKNOWN ConsoleType"
	machineTypeUnknown     = "UNKNOWN MachineType"
	stateUnknown           = "UNKNOWN State"
	volumeFormatUnknown    = "UNKNOWN VolumeFormat"
	volumeInterfaceUnknown = "UNKNOWN VolumeInterface"
	volumeTypeUnknown      = "UNKNOWN VolumeType"
	watchdogActionUnknown  = "UNKNOWN WatchdogAction"
	watchdogModelUnknown   = "UNKNOWN WatchdogModel"
)

var (
	consoleTypeToText = map[ConsoleType]string{
		ConsoleNone:  "none",
		ConsoleDummy: "dummy",
		ConsoleVNC:   "vnc",
	}
	textToConsoleType map[string]ConsoleType

	machineTypeToText = map[MachineType]string{
		MachineTypeGenericPC: "pc",
		MachineTypeQ35:       "q35",
	}
	textToMachineType map[string]MachineType

	stateToText = map[State]string{
		StateStarting:      "starting",
		StateRunning:       "running",
		StateFailedToStart: "failed to start",
		StateStopping:      "stopping",
		StateStopped:       "stopped",
		StateDestroying:    "destroying",
		StateMigrating:     "migrating",
		StateExporting:     "exporting",
		StateCrashed:       "crashed",
		StateDebugging:     "debugging",
	}
	textToState map[string]State

	volumeFormatToText = map[VolumeFormat]string{
		VolumeFormatRaw:   "raw",
		VolumeFormatQCOW2: "qcow2",
	}
	textToVolumeFormat map[string]VolumeFormat

	volumeInterfaceToText = map[VolumeInterface]string{
		VolumeInterfaceVirtIO: "virtio",
		VolumeInterfaceIDE:    "ide",
		VolumeInterfaceNVMe:   "nvme",
	}
	textToVolumeInterface map[string]VolumeInterface

	volumeTypeToText = map[VolumeType]string{
		VolumeTypePersistent: "persistent",
		VolumeTypeMemory:     "memory",
	}
	textToVolumeType map[string]VolumeType

	watchdogActionToText = map[WatchdogAction]string{
		WatchdogActionNone:     "none",
		WatchdogActionReset:    "reset",
		WatchdogActionShutdown: "shutdown",
		WatchdogActionPowerOff: "poweroff",
	}
	textToWatchdogAction map[string]WatchdogAction

	watchdogModelToText = map[WatchdogModel]string{
		WatchdogModelNone:     "none",
		WatchdogModelIb700:    "ib700",
		WatchdogModelI6300esb: "i6300esb",
	}
	textToWatchdogModel map[string]WatchdogModel
)

func init() {
	textToConsoleType = make(map[string]ConsoleType, len(consoleTypeToText))
	for consoleType, text := range consoleTypeToText {
		textToConsoleType[text] = consoleType
	}
	textToMachineType = make(map[string]MachineType, len(machineTypeToText))
	for format, text := range machineTypeToText {
		textToMachineType[text] = format
	}
	textToState = make(map[string]State, len(stateToText))
	for state, text := range stateToText {
		textToState[text] = state
	}
	textToVolumeFormat = make(map[string]VolumeFormat, len(volumeFormatToText))
	for format, text := range volumeFormatToText {
		textToVolumeFormat[text] = format
	}
	textToVolumeInterface = make(map[string]VolumeInterface,
		len(volumeInterfaceToText))
	for format, text := range volumeInterfaceToText {
		textToVolumeInterface[text] = format
	}
	textToVolumeType = make(map[string]VolumeType, len(volumeTypeToText))
	for vtype, text := range volumeTypeToText {
		textToVolumeType[text] = vtype
	}
	textToWatchdogAction = make(map[string]WatchdogAction,
		len(watchdogActionToText))
	for action, text := range watchdogActionToText {
		textToWatchdogAction[text] = action
	}
	textToWatchdogModel = make(map[string]WatchdogModel,
		len(watchdogModelToText))
	for model, text := range watchdogModelToText {
		textToWatchdogModel[text] = model
	}
}

// CompareIP returns true if the two IPs are equivalent, else false.
func CompareIPs(left, right net.IP) bool {
	if len(left) < 1 {
		if len(right) > 0 {
			return false
		}
		return true
	} else if len(right) < 1 {
		return false
	}
	return left.Equal(right)
}

func ShrinkIP(netIP net.IP) net.IP {
	switch len(netIP) {
	case 4:
		return netIP
	case 16:
		if ip4 := netIP.To4(); ip4 == nil {
			return netIP
		} else {
			return ip4
		}
	default:
		return netIP
	}
}

func (left *Address) Equal(right *Address) bool {
	if !CompareIPs(left.IpAddress, right.IpAddress) {
		return false
	}
	if left.MacAddress != right.MacAddress {
		return false
	}
	return true
}

func (address *Address) Set(value string) error {
	if split := strings.Split(value, ";"); len(split) != 2 {
		return errors.New("malformed address pair: " + value)
	} else if ip := net.ParseIP(split[1]); ip == nil {
		return errors.New("unable to parse IP: " + split[1])
	} else if ip4 := ip.To4(); ip4 == nil {
		return errors.New("address is not IPv4: " + split[1])
	} else {
		*address = Address{IpAddress: ip4, MacAddress: split[0]}
		return nil
	}
}

func (address *Address) Shrink() {
	address.IpAddress = ShrinkIP(address.IpAddress)
}

func (address *Address) String() string {
	return address.IpAddress.String() + ";" + address.MacAddress
}

func (al *AddressList) String() string {
	buffer := &bytes.Buffer{}
	buffer.WriteString(`"`)
	for index, address := range *al {
		buffer.WriteString(address.String())
		if index < len(*al)-1 {
			buffer.WriteString(",")
		}
	}
	buffer.WriteString(`"`)
	return buffer.String()
}

func (al *AddressList) Set(value string) error {
	newList := make(AddressList, 0)
	if value != "" {
		addressStrings := strings.Split(value, ",")
		for _, addressString := range addressStrings {
			var address Address
			if err := address.Set(addressString); err != nil {
				return err
			}
			newList = append(newList, address)
		}
	}
	*al = newList
	return nil
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index, leftString := range left {
		if leftString != right[index] {
			return false
		}
	}
	return true
}

func (consoleType *ConsoleType) CheckValid() error {
	if _, ok := consoleTypeToText[*consoleType]; !ok {
		return errors.New(consoleTypeUnknown)
	} else {
		return nil
	}
}

func (consoleType ConsoleType) MarshalText() ([]byte, error) {
	if text := consoleType.String(); text == consoleTypeUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (consoleType *ConsoleType) Set(value string) error {
	if val, ok := textToConsoleType[value]; !ok {
		return errors.New(consoleTypeUnknown)
	} else {
		*consoleType = val
		return nil
	}
}

func (consoleType ConsoleType) String() string {
	if str, ok := consoleTypeToText[consoleType]; !ok {
		return consoleTypeUnknown
	} else {
		return str
	}
}

func (consoleType *ConsoleType) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToConsoleType[txt]; ok {
		*consoleType = val
		return nil
	} else {
		return errors.New("unknown ConsoleType: " + txt)
	}
}

func (machineType *MachineType) CheckValid() error {
	if _, ok := machineTypeToText[*machineType]; !ok {
		return errors.New(machineTypeUnknown)
	} else {
		return nil
	}
}

func (machineType MachineType) MarshalText() ([]byte, error) {
	if text := machineType.String(); text == machineTypeUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (machineType *MachineType) Set(value string) error {
	if val, ok := textToMachineType[value]; !ok {
		return errors.New(machineTypeUnknown)
	} else {
		*machineType = val
		return nil
	}
}

func (machineType MachineType) String() string {
	if text, ok := machineTypeToText[machineType]; ok {
		return text
	} else {
		return machineTypeUnknown
	}
}

func (machineType *MachineType) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToMachineType[txt]; ok {
		*machineType = val
		return nil
	} else {
		return errors.New("unknown MachineType: " + txt)
	}
}

func (state State) MarshalText() ([]byte, error) {
	if text := state.String(); text == stateUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (state State) String() string {
	if text, ok := stateToText[state]; ok {
		return text
	} else {
		return stateUnknown
	}
}

func (state *State) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToState[txt]; ok {
		*state = val
		return nil
	} else {
		return errors.New("unknown State: " + txt)
	}
}

func (left *Subnet) Equal(right *Subnet) bool {
	if left.Id != right.Id {
		return false
	}
	if !CompareIPs(left.IpGateway, right.IpGateway) {
		return false
	}
	if !CompareIPs(left.IpMask, right.IpMask) {
		return false
	}
	if left.DomainName != right.DomainName {
		return false
	}
	if !IpListsEqual(left.DomainNameServers, right.DomainNameServers) {
		return false
	}
	if left.DisableMetadata != right.DisableMetadata {
		return false
	}
	if left.Manage != right.Manage {
		return false
	}
	if left.VlanId != right.VlanId {
		return false
	}
	if !stringSlicesEqual(left.AllowedGroups, right.AllowedGroups) {
		return false
	}
	if !stringSlicesEqual(left.AllowedUsers, right.AllowedUsers) {
		return false
	}
	if !CompareIPs(left.FirstDynamicIP, right.FirstDynamicIP) {
		return false
	}
	return true
}

func IpListsEqual(left, right []net.IP) bool {
	if len(left) != len(right) {
		return false
	}
	for index, leftIP := range left {
		if !CompareIPs(leftIP, right[index]) {
			return false
		}
	}
	return true
}

func (subnet *Subnet) Shrink() {
	subnet.IpGateway = ShrinkIP(subnet.IpGateway)
	subnet.IpMask = ShrinkIP(subnet.IpMask)
	for index, ipAddr := range subnet.DomainNameServers {
		subnet.DomainNameServers[index] = ShrinkIP(ipAddr)
	}
}

func (left *VmInfo) Equal(right *VmInfo) bool {
	if !left.Address.Equal(&right.Address) {
		return false
	}
	if !left.ChangedStateOn.Equal(right.ChangedStateOn) {
		return false
	}
	if left.ConsoleType != right.ConsoleType {
		return false
	}
	if !left.CreatedOn.Equal(right.CreatedOn) {
		return false
	}
	if left.CpuPriority != right.CpuPriority {
		return false
	}
	if left.DestroyOnPowerdown != right.DestroyOnPowerdown {
		return false
	}
	if left.DestroyProtection != right.DestroyProtection {
		return false
	}
	if left.DisableVirtIO != right.DisableVirtIO {
		return false
	}
	if left.ExtraKernelOptions != right.ExtraKernelOptions {
		return false
	}
	if left.Hostname != right.Hostname {
		return false
	}
	if !left.IdentityExpires.Equal(right.IdentityExpires) {
		return false
	}
	if left.IdentityName != right.IdentityName {
		return false
	}
	if left.ImageName != right.ImageName {
		return false
	}
	if left.ImageURL != right.ImageURL {
		return false
	}
	if left.MachineType != right.MachineType {
		return false
	}
	if left.MemoryInMiB != right.MemoryInMiB {
		return false
	}
	if left.MilliCPUs != right.MilliCPUs {
		return false
	}
	if !stringSlicesEqual(left.OwnerGroups, right.OwnerGroups) {
		return false
	}
	if !stringSlicesEqual(left.OwnerUsers, right.OwnerUsers) {
		return false
	}
	if left.SpreadVolumes != right.SpreadVolumes {
		return false
	}
	if left.State != right.State {
		return false
	}
	if !left.Tags.Equal(right.Tags) {
		return false
	}
	if len(left.SecondaryAddresses) != len(right.SecondaryAddresses) {
		return false
	}
	for index, leftAddress := range left.SecondaryAddresses {
		if !leftAddress.Equal(&right.SecondaryAddresses[index]) {
			return false
		}
	}
	if !stringSlicesEqual(left.SecondarySubnetIDs, right.SecondarySubnetIDs) {
		return false
	}
	if left.SubnetId != right.SubnetId {
		return false
	}
	if left.Uncommitted != right.Uncommitted {
		return false
	}
	if left.VirtualCPUs != right.VirtualCPUs {
		return false
	}
	if len(left.Volumes) != len(right.Volumes) {
		return false
	}
	for index, leftVolume := range left.Volumes {
		if !leftVolume.Equal(&right.Volumes[index]) {
			return false
		}
	}
	if left.WatchdogAction != right.WatchdogAction {
		return false
	}
	if left.WatchdogModel != right.WatchdogModel {
		return false
	}
	return true
}

func (left *Volume) Equal(right *Volume) bool {
	if left.Format != right.Format {
		return false
	}
	if left.Interface != right.Interface {
		return false
	}
	if left.Size != right.Size {
		return false
	}
	if len(left.Snapshots) != len(right.Snapshots) {
		return false
	}
	for name, leftSize := range left.Snapshots {
		if rightSize, ok := right.Snapshots[name]; !ok {
			return false
		} else if leftSize != rightSize {
			return false
		}
	}
	if left.Type != right.Type {
		return false
	}
	return true
}

func (volumeFormat VolumeFormat) MarshalText() ([]byte, error) {
	if text := volumeFormat.String(); text == volumeFormatUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (volumeFormat *VolumeFormat) Set(value string) error {
	if val, ok := textToVolumeFormat[value]; !ok {
		return errors.New(volumeFormatUnknown)
	} else {
		*volumeFormat = val
		return nil
	}
}

func (volumeFormat VolumeFormat) String() string {
	if text, ok := volumeFormatToText[volumeFormat]; ok {
		return text
	} else {
		return volumeFormatUnknown
	}
}

func (volumeFormat *VolumeFormat) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToVolumeFormat[txt]; ok {
		*volumeFormat = val
		return nil
	} else {
		return errors.New("unknown VolumeFormat: " + txt)
	}
}

func (volumeInterface *VolumeInterface) CheckValid() error {
	if _, ok := volumeInterfaceToText[*volumeInterface]; !ok {
		return errors.New(volumeInterfaceUnknown)
	} else {
		return nil
	}
}

func (volumeInterface VolumeInterface) MarshalText() ([]byte, error) {
	if text := volumeInterface.String(); text == volumeInterfaceUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (volumeInterface *VolumeInterface) Set(value string) error {
	if val, ok := textToVolumeInterface[value]; !ok {
		return errors.New(volumeInterfaceUnknown)
	} else {
		*volumeInterface = val
		return nil
	}
}

func (volumeInterface VolumeInterface) String() string {
	if text, ok := volumeInterfaceToText[volumeInterface]; ok {
		return text
	} else {
		return volumeInterfaceUnknown
	}
}

func (volumeInterface *VolumeInterface) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToVolumeInterface[txt]; ok {
		*volumeInterface = val
		return nil
	} else {
		return errors.New("unknown VolumeInterface: " + txt)
	}
}

func (volumeType VolumeType) MarshalText() ([]byte, error) {
	if text := volumeType.String(); text == volumeTypeUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (volumeType *VolumeType) Set(value string) error {
	if val, ok := textToVolumeType[value]; !ok {
		return errors.New(volumeTypeUnknown)
	} else {
		*volumeType = val
		return nil
	}
}

func (volumeType VolumeType) String() string {
	if text, ok := volumeTypeToText[volumeType]; ok {
		return text
	} else {
		return volumeTypeUnknown
	}
}

func (volumeType *VolumeType) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToVolumeType[txt]; ok {
		*volumeType = val
		return nil
	} else {
		return errors.New("unknown VolumeType: " + txt)
	}
}

func (watchdogAction WatchdogAction) CheckValid() error {
	if _, ok := watchdogActionToText[watchdogAction]; !ok {
		return errors.New(watchdogActionUnknown)
	} else {
		return nil
	}
}

func (watchdogAction WatchdogAction) MarshalText() ([]byte, error) {
	if text := watchdogAction.String(); text == watchdogActionUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (watchdogAction *WatchdogAction) Set(value string) error {
	if val, ok := textToWatchdogAction[value]; !ok {
		return errors.New(watchdogActionUnknown)
	} else {
		*watchdogAction = val
		return nil
	}
}

func (watchdogAction WatchdogAction) String() string {
	if text, ok := watchdogActionToText[watchdogAction]; ok {
		return text
	} else {
		return watchdogActionUnknown
	}
}

func (watchdogAction *WatchdogAction) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToWatchdogAction[txt]; ok {
		*watchdogAction = val
		return nil
	} else {
		return errors.New("unknown WatchdogAction: " + txt)
	}
}

func (watchdogModel WatchdogModel) CheckValid() error {
	if _, ok := watchdogModelToText[watchdogModel]; !ok {
		return errors.New(watchdogModelUnknown)
	} else {
		return nil
	}
}

func (watchdogModel WatchdogModel) MarshalText() ([]byte, error) {
	if text := watchdogModel.String(); text == watchdogModelUnknown {
		return nil, errors.New(text)
	} else {
		return []byte(text), nil
	}
}

func (watchdogModel *WatchdogModel) Set(value string) error {
	if val, ok := textToWatchdogModel[value]; !ok {
		return errors.New(watchdogModelUnknown)
	} else {
		*watchdogModel = val
		return nil
	}
}

func (watchdogModel WatchdogModel) String() string {
	if text, ok := watchdogModelToText[watchdogModel]; ok {
		return text
	} else {
		return watchdogModelUnknown
	}
}

func (watchdogModel *WatchdogModel) UnmarshalText(text []byte) error {
	txt := string(text)
	if val, ok := textToWatchdogModel[txt]; ok {
		*watchdogModel = val
		return nil
	} else {
		return errors.New("unknown WatchdogModel: " + txt)
	}
}
