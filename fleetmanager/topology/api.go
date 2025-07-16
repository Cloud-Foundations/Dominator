package topology

import (
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	installer_proto "github.com/Cloud-Foundations/Dominator/proto/installer"
)

type Directory struct {
	Name             string
	Directories      []*Directory        `json:",omitempty"`
	InstallConfig    *InstallConfig      `json:",omitempty"`
	Machines         []*fm_proto.Machine `json:",omitempty"`
	Subnets          []*Subnet           `json:",omitempty"`
	Tags             tags.Tags           `json:",omitempty"`
	logger           log.DebugLogger
	nameToDirectory  map[string]*Directory // Key: directory name.
	owners           *ownersType
	parent           *Directory
	path             string
	subnetIdToSubnet map[string]*Subnet // Key: subnet ID.
}

func (directory *Directory) GetPath() string {
	return directory.path
}

func (directory *Directory) Walk(fn func(*Directory) error) error {
	return directory.walk(fn)
}

type InstallConfig struct {
	StorageLayout *installer_proto.StorageLayout
}

type ownersType struct {
	OwnerGroups []string `json:",omitempty"`
	OwnerUsers  []string `json:",omitempty"`
}

type Params struct {
	Logger       log.DebugLogger
	TopologyDir  string // Directory containing topology data.
	VariablesDir string // Directory containing variables.
}

type Subnet struct {
	hyper_proto.Subnet
	FirstAutoIP     net.IP              `json:",omitempty"`
	LastAutoIP      net.IP              `json:",omitempty"`
	ReservedIPs     []net.IP            `json:",omitempty"`
	reservedIpAddrs map[string]struct{} // Key: IP address.
}

type WatchParams struct {
	Params
	CheckInterval      time.Duration
	LocalRepositoryDir string // Local directory.
	MetricsDirectory   string // Metrics namespace directory.
	TopologyRepository string // Remote Git repository.
}

func (s *Subnet) CheckIfIpIsReserved(ipAddr string) bool {
	_, ok := s.reservedIpAddrs[ipAddr]
	return ok
}

func (subnet *Subnet) Shrink() {
	subnet.shrink()
}

type Topology struct {
	Root            *Directory
	Variables       map[string]string
	hostIpAddresses map[string]struct{}
	logger          log.DebugLogger
	machineParents  map[string]*Directory // Key: machine name.
	reservedIpAddrs map[string]struct{}   // Key: IP address.
}

func Load(topologyDir string) (*Topology, error) {
	return load(Params{TopologyDir: topologyDir})
}

func LoadWithParams(params Params) (*Topology, error) {
	return load(params)
}

func Watch(topologyRepository, localRepositoryDir, topologyDir string,
	checkInterval time.Duration,
	logger log.DebugLogger) (<-chan *Topology, error) {
	return watch(WatchParams{
		Params: Params{
			Logger:      logger,
			TopologyDir: topologyDir,
		},
		CheckInterval:      checkInterval,
		LocalRepositoryDir: localRepositoryDir,
		TopologyRepository: topologyRepository,
	})
}

func WatchWithParams(params WatchParams) (<-chan *Topology, error) {
	return watch(params)
}

func (t *Topology) CheckIfIpIsHost(ipAddr string) bool {
	_, ok := t.hostIpAddresses[ipAddr]
	return ok
}

func (t *Topology) CheckIfIpIsReserved(ipAddr string) bool {
	_, ok := t.reservedIpAddrs[ipAddr]
	return ok
}

func (t *Topology) CheckIfMachineHasSubnet(name, subnetId string) (
	bool, error) {
	return t.checkIfMachineHasSubnet(name, subnetId)
}

func (t *Topology) FindDirectory(dirname string) (*Directory, error) {
	return t.findDirectory(dirname)
}

func (t *Topology) GetInstallConfigForMachine(name string) (
	*InstallConfig, error) {
	return t.getInstallConfigForMachine(name)
}

func (t *Topology) GetLocationOfMachine(name string) (string, error) {
	return t.getLocationOfMachine(name)
}

func (t *Topology) GetNumMachines() uint {
	return uint(len(t.machineParents))
}

func (t *Topology) GetSubnetsForMachine(name string) ([]*Subnet, error) {
	return t.getSubnetsForMachine(name)
}

func (t *Topology) ListMachines(dirname string) ([]*fm_proto.Machine, error) {
	return t.listMachines(dirname)
}

func (t *Topology) Walk(fn func(*Directory) error) error {
	return t.Root.Walk(fn)
}
