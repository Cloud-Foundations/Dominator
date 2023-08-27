package manager

import (
	"io"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/cachingreader"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	IdentityCertFile = "identity.cert"
	IdentityKeyFile  = "identity.key"
	UserDataFile     = "user-data.raw"
)

type addressPoolType struct {
	Free       []proto.Address
	Registered []proto.Address
}

type DhcpServer interface {
	AddLease(address proto.Address, hostname string) error
	AddSubnet(subnet proto.Subnet)
	MakeAcknowledgmentChannel(ipAddr net.IP) <-chan struct{}
	MakeRequestChannel(macAddr string) <-chan net.IP
	RemoveLease(ipAddr net.IP)
	RemoveSubnet(subnetId string)
}

type Manager struct {
	StartOptions
	healthStatusMutex sync.RWMutex
	healthStatus      string
	lockWatcher       *lockwatcher.LockWatcher
	memTotalInMiB     uint64
	notifiersMutex    sync.Mutex
	notifiers         map[<-chan proto.Update]chan<- proto.Update
	numCPUs           uint
	rootCookie        []byte
	serialNumber      string
	summaryMutex      sync.RWMutex
	summary           *summaryData
	volumeDirectories []string
	volumeInfos       map[string]volumeInfo // Key: volumeDirectory.
	mutex             sync.RWMutex          // Lock everything below (those can change).
	addressPool       addressPoolType
	disabled          bool
	objectCache       *cachingreader.ObjectServer
	ownerGroups       map[string]struct{}
	ownerUsers        map[string]struct{}
	subnets           map[string]proto.Subnet // Key: Subnet ID.
	subnetChannels    []chan<- proto.Subnet
	totalVolumeBytes  uint64
	vms               map[string]*vmInfoType // Key: IP address.
	vsocketsEnabled   bool
	uuid              string
}

type StartOptions struct {
	BridgeMap          map[string]net.Interface // Key: interface name.
	DhcpServer         DhcpServer
	ImageServerAddress string
	LockCheckInterval  time.Duration
	LockLogTimeout     time.Duration
	Logger             log.DebugLogger
	ObjectCacheBytes   uint64
	ShowVgaConsole     bool
	StateDir           string
	Username           string
	VlanIdToBridge     map[uint]string // Key: VLAN ID, value: bridge interface.
	VolumeDirectories  []string
}

type summaryData struct {
	availableMilliCPU      uint
	memUnallocated         uint64
	numFreeAddresses       uint
	numRegisteredAddresses uint
	numRunning             uint
	numStopped             uint
	numSubnets             uint
	ownerGroups            []string
	ownerUsers             []string
	updatedAt              time.Time
}

type vmInfoType struct {
	lockWatcher                *lockwatcher.LockWatcher
	mutex                      sync.RWMutex
	accessToken                []byte
	accessTokenCleanupNotifier chan<- struct{}
	commandInput               chan<- string
	commandOutput              chan byte
	destroyTimer               *time.Timer
	dirname                    string
	doNotWriteOrSend           bool
	hasHealthAgent             bool
	ipAddress                  string
	logger                     log.DebugLogger
	manager                    *Manager
	metadataChannels           map[chan<- string]struct{}
	monitorSockname            string
	ownerUsers                 map[string]struct{}
	serialInput                io.Writer
	serialOutput               chan<- byte
	stoppedNotifier            chan<- struct{}
	updating                   bool
	proto.LocalVmInfo
}

type volumeInfo struct {
	canTrim bool
}

func New(startOptions StartOptions) (*Manager, error) {
	return newManager(startOptions)
}

func (m *Manager) AcknowledgeVm(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.acknowledgeVm(ipAddr, authInfo)
}

func (m *Manager) AddAddressesToPool(addresses []proto.Address) error {
	return m.addAddressesToPool(addresses)
}

func (m *Manager) AddVmVolumes(ipAddr net.IP,
	authInfo *srpc.AuthInformation, volumeSizes []uint64) error {
	return m.addVmVolumes(ipAddr, authInfo, volumeSizes)
}

func (m *Manager) BecomePrimaryVmOwner(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.becomePrimaryVmOwner(ipAddr, authInfo)
}

func (m *Manager) ChangeOwners(ownerGroups, ownerUsers []string) error {
	return m.changeOwners(ownerGroups, ownerUsers)
}

func (m *Manager) ChangeVmConsoleType(ipAddr net.IP,
	authInfo *srpc.AuthInformation, consoleType proto.ConsoleType) error {
	return m.changeVmConsoleType(ipAddr, authInfo, consoleType)
}

func (m *Manager) ChangeVmDestroyProtection(ipAddr net.IP,
	authInfo *srpc.AuthInformation, destroyProtection bool) error {
	return m.changeVmDestroyProtection(ipAddr, authInfo, destroyProtection)
}

func (m *Manager) ChangeVmOwnerUsers(ipAddr net.IP,
	authInfo *srpc.AuthInformation, extraUsers []string) error {
	return m.changeVmOwnerUsers(ipAddr, authInfo, extraUsers)
}

func (m *Manager) ChangeVmSize(authInfo *srpc.AuthInformation,
	req proto.ChangeVmSizeRequest) error {
	return m.changeVmSize(authInfo, req)
}

func (m *Manager) ChangeVmTags(ipAddr net.IP, authInfo *srpc.AuthInformation,
	tgs tags.Tags) error {
	return m.changeVmTags(ipAddr, authInfo, tgs)
}

func (m *Manager) ChangeVmVolumeSize(ipAddr net.IP,
	authInfo *srpc.AuthInformation, index uint, size uint64) error {
	return m.changeVmVolumeSize(ipAddr, authInfo, index, size)
}

func (m *Manager) CheckOwnership(authInfo *srpc.AuthInformation) bool {
	return m.checkOwnership(authInfo)
}

func (m *Manager) CheckVmHasHealthAgent(ipAddr net.IP) (bool, error) {
	return m.checkVmHasHealthAgent(ipAddr)
}

func (m *Manager) CheckVsocketsEnabled() bool {
	return m.vsocketsEnabled
}

func (m *Manager) CloseUpdateChannel(channel <-chan proto.Update) {
	m.closeUpdateChannel(channel)
}

func (m *Manager) CommitImportedVm(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.commitImportedVm(ipAddr, authInfo)
}

func (m *Manager) ConnectToVmConsole(ipAddr net.IP,
	authInfo *srpc.AuthInformation) (net.Conn, error) {
	return m.connectToVmConsole(ipAddr, authInfo)
}

func (m *Manager) ConnectToVmManager(ipAddr net.IP) (
	chan<- byte, <-chan byte, error) {
	return m.connectToVmManager(ipAddr)
}

func (m *Manager) ConnectToVmSerialPort(ipAddr net.IP,
	authInfo *srpc.AuthInformation,
	portNumber uint) (chan<- byte, <-chan byte, error) {
	return m.connectToVmSerialPort(ipAddr, authInfo, portNumber)
}

func (m *Manager) CopyVm(conn *srpc.Conn, request proto.CopyVmRequest) error {
	return m.copyVm(conn, request)
}

func (m *Manager) CreateVm(conn *srpc.Conn) error {
	return m.createVm(conn)
}

func (m *Manager) DebugVmImage(conn *srpc.Conn,
	authInfo *srpc.AuthInformation) error {
	return m.debugVmImage(conn, authInfo)
}

func (m *Manager) DeleteVmVolume(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte, volumeIndex uint) error {
	return m.deleteVmVolume(ipAddr, authInfo, accessToken, volumeIndex)
}

func (m *Manager) DestroyVm(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte) error {
	return m.destroyVm(ipAddr, authInfo, accessToken)
}

func (m *Manager) DiscardVmAccessToken(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte) error {
	return m.discardVmAccessToken(ipAddr, authInfo, accessToken)
}

func (m *Manager) DiscardVmOldImage(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.discardVmOldImage(ipAddr, authInfo)
}

func (m *Manager) DiscardVmOldUserData(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.discardVmOldUserData(ipAddr, authInfo)
}

func (m *Manager) DiscardVmSnapshot(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.discardVmSnapshot(ipAddr, authInfo)
}

func (m *Manager) ExportLocalVm(authInfo *srpc.AuthInformation,
	request proto.ExportLocalVmRequest) (*proto.ExportLocalVmInfo, error) {
	return m.exportLocalVm(authInfo, request)
}

func (m *Manager) GetHealthStatus() string {
	return m.getHealthStatus()
}

func (m *Manager) GetImageServerAddress() string {
	return m.ImageServerAddress
}

func (m *Manager) GetNumVMs() (uint, uint) {
	return m.getNumVMs()
}

func (m *Manager) GetRootCookiePath() string {
	return filepath.Join(m.StartOptions.StateDir, "root-cookie")
}

func (m *Manager) GetVmAccessToken(ipAddr net.IP,
	authInfo *srpc.AuthInformation, lifetime time.Duration) ([]byte, error) {
	return m.getVmAccessToken(ipAddr, authInfo, lifetime)
}

func (m *Manager) GetVmBootLog(ipAddr net.IP) (io.ReadCloser, error) {
	return m.getVmBootLog(ipAddr)
}

func (m *Manager) GetVmCID(ipAddr net.IP) (uint32, error) {
	return m.getVmCID(ipAddr)
}

func (m *Manager) GetVmFileData(ipAddr net.IP, filename string) (
	io.ReadCloser, error) {
	rc, _, err := m.getVmFileReader(ipAddr,
		&srpc.AuthInformation{HaveMethodAccess: true},
		nil, filename)
	return rc, err
}

func (m *Manager) GetVmInfo(ipAddr net.IP) (proto.VmInfo, error) {
	return m.getVmInfo(ipAddr)
}

func (m *Manager) GetVmLockWatcher(ipAddr net.IP) (
	*lockwatcher.LockWatcher, error) {
	return m.getVmLockWatcher(ipAddr)
}

func (m *Manager) GetVmUserData(ipAddr net.IP) (io.ReadCloser, error) {
	rc, _, err := m.getVmFileReader(ipAddr,
		&srpc.AuthInformation{HaveMethodAccess: true},
		nil, UserDataFile)
	return rc, err
}

func (m *Manager) GetVmUserDataRPC(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte) (
	io.ReadCloser, uint64, error) {
	return m.getVmFileReader(ipAddr, authInfo, accessToken, UserDataFile)
}

func (m *Manager) GetVmVolume(conn *srpc.Conn) error {
	return m.getVmVolume(conn)
}

func (m *Manager) GetUUID() (string, error) {
	return m.uuid, nil
}

func (m *Manager) HoldLock(timeout time.Duration, writeLock bool) error {
	return m.holdLock(timeout, writeLock)
}

func (m *Manager) HoldVmLock(ipAddr net.IP, timeout time.Duration,
	writeLock bool, authInfo *srpc.AuthInformation) error {
	return m.holdVmLock(ipAddr, timeout, writeLock, authInfo)
}

func (m *Manager) ImportLocalVm(authInfo *srpc.AuthInformation,
	request proto.ImportLocalVmRequest) error {
	return m.importLocalVm(authInfo, request)
}

func (m *Manager) ListAvailableAddresses() []proto.Address {
	return m.listAvailableAddresses()
}

func (m *Manager) ListRegisteredAddresses() []proto.Address {
	return m.listRegisteredAddresses()
}

func (m *Manager) ListSubnets(doSort bool) []proto.Subnet {
	return m.listSubnets(doSort)
}

func (m *Manager) ListVMs(request proto.ListVMsRequest) []string {
	return m.listVMs(request)
}

func (m *Manager) ListVolumeDirectories() []string {
	return m.volumeDirectories
}

func (m *Manager) MakeSubnetChannel() <-chan proto.Subnet {
	return m.makeSubnetChannel()
}

func (m *Manager) MakeUpdateChannel() <-chan proto.Update {
	return m.makeUpdateChannel()
}

func (m *Manager) MigrateVm(conn *srpc.Conn) error {
	return m.migrateVm(conn)
}

func (m *Manager) NotifyVmMetadataRequest(ipAddr net.IP, path string) {
	m.notifyVmMetadataRequest(ipAddr, path)
}

func (m *Manager) PatchVmImage(conn *srpc.Conn,
	request proto.PatchVmImageRequest) error {
	return m.patchVmImage(conn, request)
}

func (m *Manager) PowerOff(stopVMs bool) error {
	return m.powerOff(stopVMs)
}

func (m *Manager) PrepareVmForMigration(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte, enable bool) error {
	return m.prepareVmForMigration(ipAddr, authInfo, accessToken, enable)
}

func (m *Manager) RebootVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	dhcpTimeout time.Duration) (bool, error) {
	return m.rebootVm(ipAddr, authInfo, dhcpTimeout)
}

func (m *Manager) RemoveAddressesFromPool(addresses []proto.Address) error {
	return m.removeAddressesFromPool(addresses)
}

func (m *Manager) RemoveExcessAddressesFromPool(maxFree map[string]uint) error {
	return m.removeExcessAddressesFromPool(maxFree)
}

func (m *Manager) RegisterVmMetadataNotifier(ipAddr net.IP,
	authInfo *srpc.AuthInformation, pathChannel chan<- string) error {
	return m.registerVmMetadataNotifier(ipAddr, authInfo, pathChannel)
}

func (m *Manager) ReplaceVmCredentials(
	request proto.ReplaceVmCredentialsRequest,
	authInfo *srpc.AuthInformation) error {
	return m.replaceVmCredentials(request, authInfo)
}

func (m *Manager) ReplaceVmImage(conn *srpc.Conn,
	authInfo *srpc.AuthInformation) error {
	return m.replaceVmImage(conn, authInfo)
}

func (m *Manager) ReplaceVmUserData(ipAddr net.IP, reader io.Reader,
	size uint64, authInfo *srpc.AuthInformation) error {
	return m.replaceVmUserData(ipAddr, reader, size, authInfo)
}

func (m *Manager) RestoreVmFromSnapshot(ipAddr net.IP,
	authInfo *srpc.AuthInformation, forceIfNotStopped bool) error {
	return m.restoreVmFromSnapshot(ipAddr, authInfo, forceIfNotStopped)
}

func (m *Manager) RestoreVmImage(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.restoreVmImage(ipAddr, authInfo)
}

func (m *Manager) RestoreVmUserData(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	return m.restoreVmUserData(ipAddr, authInfo)
}

func (m *Manager) ReorderVmVolumes(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte,
	volumeIndices []uint) error {
	return m.reorderVmVolumes(ipAddr, authInfo, accessToken, volumeIndices)
}

func (m *Manager) ScanVmRoot(ipAddr net.IP, authInfo *srpc.AuthInformation,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	return m.scanVmRoot(ipAddr, authInfo, scanFilter)
}

func (m *Manager) SetDisabledState(disable bool) error {
	return m.setDisabledState(disable)
}

func (m *Manager) ShutdownVMsAndExit() {
	m.shutdownVMsAndExit()
}

func (m *Manager) SnapshotVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	forceIfNotStopped, snapshotRootOnly bool) error {
	return m.snapshotVm(ipAddr, authInfo, forceIfNotStopped, snapshotRootOnly)
}

func (m *Manager) StartVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte, dhcpTimeout time.Duration) (
	bool, error) {
	return m.startVm(ipAddr, authInfo, accessToken, dhcpTimeout)
}

func (m *Manager) StopVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte) error {
	return m.stopVm(ipAddr, authInfo, accessToken)
}

func (m *Manager) UpdateSubnets(request proto.UpdateSubnetsRequest) error {
	return m.updateSubnets(request)
}

func (m *Manager) UnregisterVmMetadataNotifier(ipAddr net.IP,
	pathChannel chan<- string) error {
	return m.unregisterVmMetadataNotifier(ipAddr, pathChannel)
}

func (m *Manager) WriteHtml(writer io.Writer) {
	m.writeHtml(writer)
}
