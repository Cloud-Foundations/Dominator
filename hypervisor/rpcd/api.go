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

type Config struct {
	AllowUnauthenticatedReads bool
}

type Params struct {
	DhcpServer     DhcpServer
	Logger         log.DebugLogger
	Manager        *manager.Manager
	TftpbootServer TftpbootServer
}

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

func Setup(config Config, params Params) (*htmlWriter, error) {
	srpcObj := &srpcType{
		dhcpServer:     params.DhcpServer,
		logger:         params.Logger,
		manager:        params.Manager,
		tftpbootServer: params.TftpbootServer,
		externalLeases: make(map[ipv4Address]string),
	}
	srpc.SetDefaultGrantMethod(
		func(_ string, authInfo *srpc.AuthInformation) bool {
			return params.Manager.CheckOwnership(authInfo)
		})
	publicMethods := []string{
		"AcknowledgeVm",
		"AddVmVolumes",
		"BecomePrimaryVmOwner",
		"ChangeVmConsoleType",
		"ChangeVmCpuPriority",
		"ChangeVmDestroyProtection",
		"ChangeVmHostname",
		"ChangeVmMachineType",
		"ChangeVmNumNetworkQueues",
		"ChangeVmOwnerGroups",
		"ChangeVmOwnerUsers",
		"ChangeVmSize",
		"ChangeVmSubnet",
		"ChangeVmTags",
		"ChangeVmVolumeInterfaces",
		"ChangeVmVolumeSize",
		"ChangeVmVolumeStorageIndex",
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
		"GetVmCreateRequest",
		"GetVmInfo",
		"GetVmInfos",
		"GetVmLastPatchLog",
		"GetVmUserData",
		"GetVmVirtualiserLogFile",
		"GetVmVolume",
		"GetVmVolumeStorageConfiguration",
		"ImportLocalVm",
		"ListSubnets",
		"ListVMs",
		"ListVmVirtualiserLogFiles",
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
	}
	var unauthenticatedMethods []string
	if config.AllowUnauthenticatedReads {
		unauthenticatedMethods = []string{
			"GetCapacity",
			"GetVmInfo",
			"GetVmInfos",
			"GetVmLastPatchLog",
			"ListSubnets",
			"ListVMs",
		}
	}
	srpc.RegisterNameWithOptions("Hypervisor", srpcObj, srpc.ReceiverOptions{
		PublicMethods:          publicMethods,
		UnauthenticatedMethods: unauthenticatedMethods,
	})
	return (*htmlWriter)(srpcObj), nil
}
