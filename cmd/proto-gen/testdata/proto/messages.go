package hypervisor

import (
	"net"

	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

type Address struct {
	IpAddress  net.IP
	MacAddress string
}

type GetVmInfoRequest struct {
	IpAddress net.IP
}

type GetVmInfoResponse struct {
	VmInfo VmInfo
	Error  string
}

type ListVMsRequest struct {
	OwnerUsers []string
	Sort       bool `proto:"3"`
	Tags       tags.Tags
	Hidden     string `proto:"-"`
}

type ListVMsResponse struct {
	IpAddresses []net.IP
	Error       string
}

// Exposed although no method uses it.
// @grpc
type Standalone struct {
	Name string
}

type Blob []byte

type Labels tags.Tags

type State uint

type VmInfo struct {
	Address     Address
	Hostname    string `proto:"5"`
	Secondary   []*Address
	State       State
	Volumes     map[string]Address
	First, Next uint64
}
