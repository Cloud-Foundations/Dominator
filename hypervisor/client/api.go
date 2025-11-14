package client

import (
	"io"
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type FlushReadWriter interface {
	Flush() error
	io.ReadWriter
}

type GetUpdatesParams struct {
	ConnectedHandler func() error
	Logger           log.DebugLogger
	RequestsChannel  <-chan proto.GetUpdatesRequest
	UpdateHandler    func(update proto.Update) error
}

func AcknowledgeVm(client srpc.ClientI, ipAddress net.IP) error {
	return acknowledgeVm(client, ipAddress)
}

func AddVmVolumes(client srpc.ClientI, ipAddress net.IP, sizes []uint64) error {
	return addVmVolumes(client, ipAddress, sizes)
}

func BecomePrimaryVmOwner(client srpc.ClientI, ipAddress net.IP) error {
	return becomePrimaryVmOwner(client, ipAddress)
}

func ChangeVmConsoleType(client srpc.ClientI, ipAddress net.IP,
	consoleType proto.ConsoleType) error {
	return changeVmConsoleType(client, ipAddress, consoleType)
}

func ChangeVmCpuPriority(client srpc.ClientI,
	request proto.ChangeVmCpuPriorityRequest) error {
	return changeVmCpuPriority(client, request)
}

func ChangeVmDestroyProtection(client srpc.ClientI, ipAddress net.IP,
	destroyProtection bool) error {
	return changeVmDestroyProtection(client, ipAddress, destroyProtection)
}

func ChangeVmHostname(client srpc.ClientI, ipAddress net.IP,
	hostname string) error {
	return changeVmHostname(client, ipAddress, hostname)
}

func ChangeVmMachineType(client srpc.ClientI, ipAddress net.IP,
	machineType proto.MachineType) error {
	return changeVmMachineType(client, ipAddress, machineType)
}

func ChangeVmOwnerGroups(client srpc.ClientI, ipAddress net.IP,
	ownerGroups []string) error {
	return changeVmOwnerGroups(client, ipAddress, ownerGroups)
}

func ChangeVmOwnerUsers(client srpc.ClientI, ipAddress net.IP,
	ownerUsers []string) error {
	return changeVmOwnerUsers(client, ipAddress, ownerUsers)
}

func ChangeVmSize(client srpc.ClientI,
	request proto.ChangeVmSizeRequest) error {
	return changeVmSize(client, request)
}

func ChangeVmSubnet(client srpc.ClientI,
	request proto.ChangeVmSubnetRequest) (proto.ChangeVmSubnetResponse, error) {
	return changeVmSubnet(client, request)
}

func ChangeVmTags(client srpc.ClientI, ipAddress net.IP, tgs tags.Tags) error {
	return changeVmTags(client, ipAddress, tgs)
}

func ChangeVmVolumeInterfaces(client srpc.ClientI, ipAddress net.IP,
	volumeInterfaces []proto.VolumeInterface) error {
	return changeVmVolumeInterfaces(client, ipAddress, volumeInterfaces)
}

func ChangeVmVolumeSize(client srpc.ClientI, ipAddress net.IP, index uint,
	size uint64) error {
	return changeVmVolumeSize(client, ipAddress, index, size)
}

func ChangeVmVolumeStorageIndex(client srpc.ClientI, ipAddress net.IP,
	storageIndex, volumeIndex uint) error {
	return changeVmVolumeStorageIndex(client, ipAddress, storageIndex,
		volumeIndex)
}

func CommitImportedVm(client srpc.ClientI, ipAddress net.IP) error {
	return commitImportedVm(client, ipAddress)
}

func ConnectToVmConsole(client srpc.ClientI, ipAddress net.IP,
	vncViewerCommand string, logger log.DebugLogger) error {
	return connectToVmConsole(client, ipAddress, vncViewerCommand, logger)
}

func ConnectToVmManager(hypervisorAddress string, ipAddress net.IP,
	connectionHandler func(conn FlushReadWriter) error) error {
	return connectToVmManager(hypervisorAddress, ipAddress, connectionHandler)
}

func ConnectToVmSerialPort(hypervisorAddress string, ipAddress net.IP,
	serialPortNumber uint,
	connectionHandler func(conn FlushReadWriter) error) error {
	return connectToVmSerialPort(hypervisorAddress, ipAddress, serialPortNumber,
		connectionHandler)
}

func CopyVm(client srpc.ClientI, request proto.CopyVmRequest,
	logger log.DebugLogger) (proto.CopyVmResponse, error) {
	return copyVm(client, request, logger)
}

func CreateVm(client srpc.ClientI, request proto.CreateVmRequest,
	reply *proto.CreateVmResponse, logger log.DebugLogger) error {
	return createVm(client, request, reply, logger)
}

func DebugVmImage(client srpc.ClientI, request proto.DebugVmImageRequest,
	imageReader io.Reader, imageSize int64,
	logger log.DebugLogger) (bool, error) {
	return debugVmImage(client, request, imageReader, imageSize, logger)
}

func DeleteVmVolume(client srpc.ClientI, ipAddress net.IP, accessToken []byte,
	volumeIndex uint) error {
	return deleteVmVolume(client, ipAddress, accessToken, volumeIndex)
}

func DestroyVm(client srpc.ClientI, ipAddress net.IP,
	accessToken []byte) error {
	return destroyVm(client, ipAddress, accessToken)
}

func DiscardVmAccessToken(client srpc.ClientI, ipAddress net.IP,
	token []byte) error {
	return discardVmAccessToken(client, ipAddress, token)
}

func DiscardVmOldImage(client srpc.ClientI, ipAddress net.IP) error {
	return discardVmOldImage(client, ipAddress)
}

func DiscardVmOldUserData(client srpc.ClientI, ipAddress net.IP) error {
	return discardVmOldUserData(client, ipAddress)
}

func DiscardVmSnapshot(client srpc.ClientI, ipAddress net.IP,
	name string) error {
	return discardVmSnapshot(client, ipAddress, name)
}

func ExportLocalVm(client srpc.ClientI, ipAddress net.IP,
	verificationCookie []byte) (proto.ExportLocalVmInfo, error) {
	return exportLocalVm(client, ipAddress, verificationCookie)
}

func GetCapacity(client srpc.ClientI) (proto.GetCapacityResponse, error) {
	return getCapacity(client)
}

// GetIdentityProvider will get the base URL of the Identity Provider.
func GetIdentityProvider(client srpc.ClientI) (string, error) {
	return getIdentityProvider(client)
}

// GetPublicKey will get the PEM-encoded public key for the Hypervisor.
func GetPublicKey(client srpc.ClientI) ([]byte, error) {
	return getPublicKey(client)
}

func GetRootCookiePath(client srpc.ClientI) (string, error) {
	return getRootCookiePath(client)
}

func GetUpdates(client srpc.ClientI, params GetUpdatesParams) error {
	return getUpdates(client, params)
}

func GetVmAccessToken(client srpc.ClientI, ipAddress net.IP,
	lifetime time.Duration) ([]byte, error) {
	return getVmAccessToken(client, ipAddress, lifetime)
}

func GetVmCreateRequest(client srpc.ClientI, ipAddress net.IP) (
	proto.CreateVmRequest, error) {
	return getVmCreateRequest(client, ipAddress)
}

func GetVmInfo(client srpc.ClientI, ipAddress net.IP) (proto.VmInfo, error) {
	return getVmInfo(client, ipAddress)
}

func GetVmInfos(client srpc.ClientI,
	request proto.GetVmInfosRequest) ([]proto.VmInfo, error) {
	return getVmInfos(client, request)
}

func GetVmLastPatchLog(client srpc.ClientI, ipAddress net.IP) (
	[]byte, time.Time, error) {
	return getVmLastPatchLog(client, ipAddress)
}

func GetVmUserData(client srpc.ClientI, ipAddress net.IP,
	accessToken []byte) (io.ReadCloser, uint64, error) {
	return getVmUserData(client, ipAddress, accessToken)
}

func GetVmVolume(client srpc.ClientI, request proto.GetVmVolumeRequest,
	writer io.WriteSeeker, reader io.Reader, initialFileSize, size uint64,
	logger log.DebugLogger) (proto.GetVmVolumeResponse, error) {
	return getVmVolume(client, request, writer, reader, initialFileSize, size,
		logger)
}

func GetVmVolumeStorageConfiguration(client srpc.ClientI, ipAddress net.IP) (
	proto.GetVmVolumeStorageConfigurationResponse, error) {
	return getVmVolumeStorageConfiguration(client, ipAddress)
}

func HoldLock(client srpc.ClientI, timeout time.Duration,
	writeLock bool) error {
	return holdLock(client, timeout, writeLock)
}

func HoldVmLock(client srpc.ClientI, ipAddress net.IP, timeout time.Duration,
	writeLock bool) error {
	return holdVmLock(client, ipAddress, timeout, writeLock)
}

func ImportLocalVm(client srpc.ClientI,
	request proto.ImportLocalVmRequest) error {
	return importLocalVm(client, request)
}

func ListSubnets(client srpc.ClientI, doSort bool) ([]proto.Subnet, error) {
	return listSubnets(client, doSort)
}

func ListVMs(client srpc.ClientI,
	request proto.ListVMsRequest) ([]net.IP, error) {
	return listVMs(client, request)
}

func ListVolumeDirectories(client srpc.ClientI, doSort bool) ([]string, error) {
	return listVolumeDirectories(client, doSort)
}

func MigrateVm(client srpc.ClientI, request proto.MigrateVmRequest,
	commitFunc func() bool, logger log.DebugLogger) error {
	return migrateVm(client, request, commitFunc, logger)
}

func OpenCreateVmConn(client srpc.ClientI, request proto.CreateVmRequest) (
	*srpc.Conn, error) {
	return openCreateVmConn(client, request)
}

func PatchVmImage(client srpc.ClientI, request proto.PatchVmImageRequest,
	logger log.DebugLogger) (bool, error) {
	return patchVmImage(client, request, logger)
}

func PowerOff(client srpc.ClientI, stopVMs bool) error {
	return powerOff(client, stopVMs)
}

func PrepareVmForMigration(client srpc.ClientI, ipAddress net.IP,
	accessToken []byte, enable bool) error {
	return prepareVmForMigration(client, ipAddress, accessToken, enable)
}

func ProbeVmPort(client srpc.ClientI, request proto.ProbeVmPortRequest) (
	proto.ProbeVmPortResponse, error) {
	return probeVmPort(client, request)
}

func ProcessCreateVmResponses(conn *srpc.Conn,
	logger log.DebugLogger) (proto.CreateVmResponse, error) {
	return processCreateVmResponses(conn, logger)
}

func RebootVm(client srpc.ClientI, ipAddress net.IP,
	dhcpTimeout time.Duration) (bool, error) {
	return rebootVm(client, ipAddress, dhcpTimeout)
}

func RegisterExternalLeases(client srpc.ClientI, addressList proto.AddressList,
	hostnames []string) error {
	return registerExternalLeases(client, addressList, hostnames)
}

func ReorderVmVolumes(client srpc.ClientI, ipAddress net.IP, accessToken []byte,
	volumeIndices []uint) error {
	return reorderVmVolumes(client, ipAddress, accessToken, volumeIndices)
}

func ReplaceVmCredentials(client srpc.ClientI,
	request proto.ReplaceVmCredentialsRequest) error {
	return replaceVmCredentials(client, request)
}

func ReplaceVmIdentity(client srpc.ClientI,
	request proto.ReplaceVmIdentityRequest) error {
	return replaceVmIdentity(client, request)
}

func ReplaceVmImage(client srpc.ClientI, request proto.ReplaceVmImageRequest,
	imageReader io.Reader, logger log.DebugLogger) (bool, error) {
	return replaceVmImage(client, request, imageReader, logger)
}

func ReplaceVmUserData(client srpc.ClientI, ipAddress net.IP,
	userData io.Reader, size uint64, logger log.DebugLogger) error {
	return replaceVmUserData(client, ipAddress, userData, size, logger)
}

func RestoreVmFromSnapshot(client srpc.ClientI,
	request proto.RestoreVmFromSnapshotRequest) error {
	return restoreVmFromSnapshot(client, request)
}

func RestoreVmImage(client srpc.ClientI,
	request proto.RestoreVmImageRequest) error {
	return restoreVmImage(client, request)
}

func RestoreVmUserData(client srpc.ClientI, ipAddress net.IP) error {
	return restoreVmUserData(client, ipAddress)
}

func ScanVmRoot(client srpc.ClientI, ipAddress net.IP,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	return scanVmRoot(client, ipAddress, scanFilter)
}

func SetDisabledState(client srpc.ClientI, disable bool) error {
	return setDisabledState(client, disable)
}

func SnapshotVm(client srpc.ClientI,
	request proto.SnapshotVmRequest) error {
	return snapshotVm(client, request)
}

func StartVm(client srpc.ClientI, ipAddress net.IP, accessToken []byte) error {
	_, err := startVm(client, ipAddress, accessToken, 0)
	return err
}

func StartVmDhcpTimeout(client srpc.ClientI, ipAddress net.IP,
	accessToken []byte, dhcpTimeout time.Duration) (bool, error) {
	return startVm(client, ipAddress, accessToken, dhcpTimeout)
}

func StopVm(client srpc.ClientI, ipAddress net.IP, accessToken []byte) error {
	return stopVm(client, ipAddress, accessToken)
}

func TraceVmMetadata(client srpc.ClientI, ipAddress net.IP,
	pathHandler func(path string) error) error {
	return traceVmMetadata(client, ipAddress, pathHandler)
}

func WatchDhcp(client srpc.ClientI, request proto.WatchDhcpRequest,
	handlePacket func(ifName string, rawPacket []byte) error) error {
	return watchDhcp(client, request, handlePacket)
}
