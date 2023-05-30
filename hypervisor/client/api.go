package client

import (
	"net"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func AcknowledgeVm(client *srpc.Client, ipAddress net.IP) error {
	return acknowledgeVm(client, ipAddress)
}

func AddVmVolumes(client *srpc.Client, ipAddress net.IP, sizes []uint64) error {
	return addVmVolumes(client, ipAddress, sizes)
}

func ChangeVmSize(client *srpc.Client,
	request proto.ChangeVmSizeRequest) error {
	return changeVmSize(client, request)
}

func ChangeVmVolumeSize(client *srpc.Client, ipAddress net.IP, index uint,
	size uint64) error {
	return changeVmVolumeSize(client, ipAddress, index, size)
}

func ConnectToVmConsole(client *srpc.Client, ipAddr net.IP,
	vncViewerCommand string, logger log.DebugLogger) error {
	return connectToVmConsole(client, ipAddr, vncViewerCommand, logger)
}

func CreateVm(client *srpc.Client, request proto.CreateVmRequest,
	reply *proto.CreateVmResponse, logger log.DebugLogger) error {
	return createVm(client, request, reply, logger)
}

func DeleteVmVolume(client *srpc.Client, ipAddr net.IP, accessToken []byte,
	volumeIndex uint) error {
	return deleteVmVolume(client, ipAddr, accessToken, volumeIndex)
}

func DestroyVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
	return destroyVm(client, ipAddr, accessToken)
}

func ExportLocalVm(client *srpc.Client, ipAddr net.IP,
	verificationCookie []byte) (proto.ExportLocalVmInfo, error) {
	return exportLocalVm(client, ipAddr, verificationCookie)
}

func GetRootCookiePath(client *srpc.Client) (string, error) {
	return getRootCookiePath(client)
}

func GetVmInfo(client *srpc.Client, ipAddr net.IP) (proto.VmInfo, error) {
	return getVmInfo(client, ipAddr)
}

func HoldLock(client *srpc.Client, timeout time.Duration,
	writeLock bool) error {
	return holdLock(client, timeout, writeLock)
}

func HoldVmLock(client *srpc.Client, ipAddr net.IP, timeout time.Duration,
	writeLock bool) error {
	return holdVmLock(client, ipAddr, timeout, writeLock)
}

func ListSubnets(client *srpc.Client, doSort bool) ([]proto.Subnet, error) {
	return listSubnets(client, doSort)
}

func PowerOff(client *srpc.Client, stopVMs bool) error {
	return powerOff(client, stopVMs)
}

func PrepareVmForMigration(client *srpc.Client, ipAddr net.IP,
	accessToken []byte, enable bool) error {
	return prepareVmForMigration(client, ipAddr, accessToken, enable)
}

func RegisterExternalLeases(client *srpc.Client, addressList proto.AddressList,
	hostnames []string) error {
	return registerExternalLeases(client, addressList, hostnames)
}

func ReorderVmVolumes(client *srpc.Client, ipAddr net.IP, accessToken []byte,
	volumeIndices []uint) error {
	return reorderVmVolumes(client, ipAddr, accessToken, volumeIndices)
}

func ScanVmRoot(client *srpc.Client, ipAddr net.IP,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	return scanVmRoot(client, ipAddr, scanFilter)
}

func StartVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
	return startVm(client, ipAddr, accessToken)
}

func StopVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
	return stopVm(client, ipAddr, accessToken)
}
