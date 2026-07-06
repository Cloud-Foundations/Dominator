package fleetmanager

import (
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	AllocationRequestError         = AllocationDeletionReason(0)
	AllocationRequestCompleted     = AllocationDeletionReason(1)
	AllocationRequestCancelled     = AllocationDeletionReason(2)
	AllocationRequestCannotFit     = AllocationDeletionReason(3)
	AllocationRequestExpired       = AllocationDeletionReason(4)
	AllocationRequestCreateTimeout = AllocationDeletionReason(5)
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
	ArchitectureType      proto.ArchitectureType
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
	HypervisorData
	Machine
	VMs []proto.VmInfo `json:",omitempty"`
}

type HypervisorData struct {
	AllocatedMilliCPUs   uint64          `json:",omitempty"`
	AllocatedMemory      uint64          `json:",omitempty"` // MiB.
	AllocatedVolumeBytes uint64          `json:",omitempty"`
	AvailableMemory      uint64          `json:",omitempty"` // MiB.
	NumFreeAddresses     map[string]uint `json:",omitempty"` // Key: subnet ID.
}

type GetIpInfoRequest struct {
	IpAddress net.IP
}

type GetIpInfoResponse struct {
	HypervisorAddress string // host:port
	Error             string
	VM                *proto.VmInfo `json:",omitempty"`
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
	ChangedHypervisors map[string]HypervisorData `json:",omitempty"` // Key: hostname.
	ChangedMachines    []*Machine                `json:",omitempty"`
	ChangedVMs         map[string]*proto.VmInfo  `json:",omitempty"` // Key: IPaddr
	DeletedMachines    []string                  `json:",omitempty"` // Hostname
	DeletedVMs         []string                  `json:",omitempty"` // IPaddr
	Error              string                    `json:",omitempty"`
	VmToHypervisor     map[string]string         `json:",omitempty"` // IP:hostname
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
	ArchitectureType      proto.ArchitectureType
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
	MachineData
	GatewaySubnetId         string       `json:",omitempty"`
	IPMI                    NetworkEntry `json:",omitempty"`
	Location                string       `json:",omitempty"`
	NetworkEntry            `json:",omitempty"`
	OwnerGroups             []string       `json:",omitempty"`
	OwnerUsers              []string       `json:",omitempty"`
	SecondaryNetworkEntries []NetworkEntry `json:",omitempty"`
	Tags                    tags.Tags      `json:",omitempty"`
}

type MachineData struct {
	ArchitectureType proto.ArchitectureType `json:",omitempty"`
	MemoryInMiB      uint64                 `json:",omitempty"`
	NumCPUs          uint                   `json:",omitempty"`
	TotalVolumeBytes uint64                 `json:",omitempty"`
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

// The Allocation RPC is experimental and subject to change without notice.
type AllocateRequest struct {
	Deadline time.Time
	// Placement
	// Priority uint
	VMs []VmAllocationSpecification
}

type AllocateResponse struct {
	Error          string
	RequestId      RequestId
	UpdatePosition uint64
}

// The CancelAllocation RPC is experimental and subject to change without
// notice.
type CancelAllocationRequest struct {
	RequestId RequestId
}

type CancelAllocationResponse struct {
	Error string
}

// The GetAllocationUpdates() RPC is fully streamed.
// The client sends a single GetAllocationUpdatesRequest message.
// The server sends a stream of AllocationUpdate messages.
// This RPC is experimental and subject to change without notice.

type GetAllocationUpdatesRequest struct {
	IncludeRequests bool      // If true: include original allocation requests.
	Position        uint64    // The position of the first update to receive.
	MaxUpdates      uint64    // Zero means infinite.
	UntilRequestId  RequestId // Empty means infinite.
}

type AllocationUpdate struct {
	AllocationUpdateEntry
	Error    string `json:",omitempty"`
	Position uint64
}
