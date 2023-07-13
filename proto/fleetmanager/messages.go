package fleetmanager

import (
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type ChangeMachineTagsRequest struct {
	Hostname string
	Tags     tags.Tags
}

type ChangeMachineTagsResponse struct {
	Error string
}

type GetHypervisorForVMRequest struct {
	IpAddress net.IP
}

type GetHypervisorForVMResponse struct {
	HypervisorAddress string // host:port
	Error             string
}

type GetHypervisorsInLocationRequest struct {
	HypervisorTagsToMatch tags.MatchTags // Empty: match all tags.
	IncludeUnhealthy      bool
	IncludeVMs            bool
	Location              string
	SubnetId              string
}

type GetHypervisorsInLocationResponse struct {
	Error       string
	Hypervisors []Hypervisor `json:",omitempty"`
}

type Hypervisor struct {
	AllocatedMilliCPUs   uint64 `json:",omitempty"`
	AllocatedMemory      uint64 `json:",omitempty"`
	AllocatedVolumeBytes uint64 `json:",omitempty"`
	Machine
	VMs []proto.VmInfo `json:",omitempty"`
}

type GetMachineInfoRequest struct {
	Hostname               string
	IgnoreMissingLocalTags bool
}

type GetMachineInfoResponse struct {
	Error    string          `json:",omitempty"`
	Location string          `json:",omitempty"`
	Machine  Machine         `json:",omitempty"`
	Subnets  []*proto.Subnet `json:",omitempty"`
}

// The GetUpdates() RPC is fully streamed.
// The client sends a single GetUpdatesRequest message.
// The server sends a stream of Update messages.

type GetUpdatesRequest struct {
	IgnoreMissingLocalTags bool
	Location               string
	MaxUpdates             uint64 // Zero means infinite.
}

type Update struct {
	ChangedMachines []*Machine               `json:",omitempty"`
	ChangedVMs      map[string]*proto.VmInfo `json:",omitempty"` // Key: IPaddr
	DeletedMachines []string                 `json:",omitempty"` // Hostname
	DeletedVMs      []string                 `json:",omitempty"` // IPaddr
	Error           string                   `json:",omitempty"`
	VmToHypervisor  map[string]string        `json:",omitempty"` // IP:hostname
}

type HardwareAddr net.HardwareAddr

type ListHypervisorLocationsRequest struct {
	TopLocation string
}

type ListHypervisorLocationsResponse struct {
	Locations []string
	Error     string
}

type ListHypervisorsInLocationRequest struct {
	HypervisorTagsToMatch tags.MatchTags // Empty: match all tags.
	IncludeUnhealthy      bool
	Location              string
	SubnetId              string
	TagsToInclude         []string
}

type ListHypervisorsInLocationResponse struct {
	Error               string
	HypervisorAddresses []string    // host:port
	TagsForHypervisors  []tags.Tags `json:",omitempty"`
}

type ListVMsInLocationRequest struct {
	HypervisorTagsToMatch tags.MatchTags // Empty: match all tags.
	Location              string
	OwnerGroups           []string
	OwnerUsers            []string
	VmTagsToMatch         tags.MatchTags // Empty: match all tags.
}

// A stream of ListVMsInLocationResponse messages is sent, until either the
// length of the IpAddresses field is zero, or the Error field != "".
type ListVMsInLocationResponse struct {
	IpAddresses []net.IP
	Error       string
}

type Machine struct {
	GatewaySubnetId         string `json:",omitempty"`
	Location                string `json:",omitempty"`
	MemoryInMiB             uint64 `json:",omitempty"`
	NetworkEntry            `json:",omitempty"`
	NumCPUs                 uint           `json:",omitempty"`
	IPMI                    NetworkEntry   `json:",omitempty"`
	OwnerGroups             []string       `json:",omitempty"`
	OwnerUsers              []string       `json:",omitempty"`
	SecondaryNetworkEntries []NetworkEntry `json:",omitempty"`
	Tags                    tags.Tags      `json:",omitempty"`
	TotalVolumeBytes        uint64         `json:",omitempty"`
}

type MoveIpAddressesRequest struct {
	HypervisorHostname string
	IpAddresses        []net.IP
}

type MoveIpAddressesResponse struct {
	Error string
}

type NetworkEntry struct {
	Hostname       string       `json:",omitempty"`
	HostIpAddress  net.IP       `json:",omitempty"`
	HostMacAddress HardwareAddr `json:",omitempty"`
	SubnetId       string       `json:",omitempty"`
	VlanTrunk      bool         `json:",omitempty"`
}

type PowerOnMachineRequest struct {
	Hostname string
}

type PowerOnMachineResponse struct {
	Error string
}
