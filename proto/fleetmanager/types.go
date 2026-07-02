package fleetmanager

import (
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/tags"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

// This type is experimental and subject to change without notice.
type AllocationDeletionReason uint

// This type is experimental and subject to change without notice.
type AllocateRequestEntry struct {
	Request   AllocateRequest
	RequestId RequestId
	Username  types.Username
}

// This type is experimental and subject to change without notice.
type Allocation struct {
	CreateDeadline time.Time
	VMs            []VmAllocation
}

// This type is experimental and subject to change without notice.
type AllocationUpdateEntry struct {
	Available *Allocation        `json:",omitempty"`
	Deleted   *DeletedAllocation `json:",omitempty"`
	Request   *AllocateRequest   `json:",omitempty"` // Not for deleted allocs.
	RequestId RequestId
	Timestamp time.Time
	Username  types.Username
}

// This type is experimental and subject to change without notice.
type DeletedAllocation struct {
	Error  string                   `json:",omitempty"`
	Reason AllocationDeletionReason `json:",omitempty"`
}

// This type is experimental and subject to change without notice.
type NetworkInterfaceSpecification struct {
	SubnetId string
}

// This type is experimental and subject to change without notice.
type RequestId string

// This type is experimental and subject to change without notice.
type VmAllocation struct {
	HypervisorAddress string // host:port
}

// This type is experimental and subject to change without notice.
type VmAllocationSpecification struct {
	HypervisorArchitecture hyper_proto.ArchitectureType
	HypervisorTagsToMatch  tags.MatchTags // Empty: match all tags.
	Location               string         `json:",omitempty"`
	MemoryInMiB            uint64
	MilliCPUs              uint
	NetworkInterfaces      []NetworkInterfaceSpecification
	Volumes                []VolumeSpecification
}

// This type is experimental and subject to change without notice.
type VolumeSpecification struct {
	Size types.Bytes
	Type hyper_proto.VolumeType
}
