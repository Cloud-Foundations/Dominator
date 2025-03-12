package rpcd

import (
	"io"
	"net"
	"sync"

	"github.com/Cloud-Foundations/Dominator/hypervisor/manager"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type DhcpServer interface {
	AddLease(address proto.Address, hostname string) error
	AddNetbootLease(address proto.Address, hostname string,
		subnet *proto.Subnet) error
	ClosePacketWatchChannel(channel <-chan proto.WatchDhcpResponse)
	MakeAcknowledgmentChannel(ipAddr net.IP) <-chan struct{}
	MakePacketWatchChannel() <-chan proto.WatchDhcpResponse
	RemoveLease(ipAddr net.IP)
}

type ipv4Address [4]byte

type srpcType struct {
	dhcpServer           DhcpServer
	logger               log.DebugLogger
	manager              *manager.Manager
	tftpbootServer       TftpbootServer
	mutex                sync.Mutex             // Protect everything below.
	externalLeases       map[ipv4Address]string // Value: MAC address.
	manageExternalLeases bool
}

type TftpbootServer interface {
	RegisterFiles(ipAddr net.IP, files map[string][]byte)
	UnregisterFiles(ipAddr net.IP)
}

type htmlWriter srpcType

func (hw *htmlWriter) WriteHtml(writer io.Writer) {
	hw.writeHtml(writer)
}

func Setup(manager *manager.Manager, dhcpServer DhcpServer,
	tftpbootServer TftpbootServer, logger log.DebugLogger) (
	*htmlWriter, error) {
	srpcObj := &srpcType{
		dhcpServer:     dhcpServer,
		logger:         logger,
		manager:        manager,
		tftpbootServer: tftpbootServer,
		externalLeases: make(map[ipv4Address]string),
	}
	srpc.SetDefaultGrantMethod(
		func(_ string, authInfo *srpc.AuthInformation) bool {
			return manager.CheckOwnership(authInfo)
		})
	srpc.RegisterNameWithOptions("Hypervisor", srpcObj, srpc.ReceiverOptions{
		PublicMethods: []string{
			"AcknowledgeVm",
			"AddVmVolumes",
			"BecomePrimaryVmOwner",
			"ChangeVmConsoleType",
			"ChangeVmCpuPriority",
			"ChangeVmDestroyProtection",
			"ChangeVmMachineType",
			"ChangeVmOwnerGroups",
			"ChangeVmOwnerUsers",
			"ChangeVmSize",
			"ChangeVmTags",
			"ChangeVmVolumeInterfaces",
			"ChangeVmVolumeSize",
			"CommitImportedVm",
			"ConnectToVmConsole",
			"ConnectToVmSerialPort",
			"CopyVm",
			"CreateVm",
			"DebugVmImage",
			"DeleteVmVolume",
			"DestroyVm",
			"DiscardVmAccessToken",
			"DiscardVmOldImage",
			"DiscardVmOldUserData",
			"DiscardVmSnapshot",
			"ExportLocalVm",
			"GetCapacity",
			"GetIdentityProvider",
			"GetPublicKey",
			"GetRootCookiePath",
			"GetUpdates",
			"GetVmAccessToken",
			"GetVmInfo",
			"GetVmInfos",
			"GetVmLastPatchLog",
			"GetVmUserData",
			"GetVmVolume",
			"ImportLocalVm",
			"ListSubnets",
			"ListVMs",
			"ListVolumeDirectories",
			"MigrateVm",
			"PatchVmImage",
			"ProbeVmPort",
			"RebootVm",
			"ReplaceVmCredentials",
			"ReplaceVmIdentity",
			"ReplaceVmImage",
			"ReplaceVmUserData",
			"RestoreVmFromSnapshot",
			"RestoreVmImage",
			"RestoreVmUserData",
			"ReorderVmVolumes",
			"ScanVmRoot",
			"SnapshotVm",
			"StartVm",
			"StopVm",
			"TraceVmMetadata",
		}})
	return (*htmlWriter)(srpcObj), nil
}
