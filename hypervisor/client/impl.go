package client

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/bufwriter"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func acknowledgeVm(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.AcknowledgeVmRequest{ipAddress}
	var reply proto.AcknowledgeVmResponse
	return client.RequestReply("Hypervisor.AcknowledgeVm", request, &reply)
}

func addVmVolumes(client srpc.ClientI, ipAddress net.IP, sizes []uint64) error {
	request := proto.AddVmVolumesRequest{
		IpAddress:   ipAddress,
		VolumeSizes: sizes,
	}
	var reply proto.AddVmVolumesResponse
	err := client.RequestReply("Hypervisor.AddVmVolumes", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func becomePrimaryVmOwner(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.BecomePrimaryVmOwnerRequest{ipAddress}
	var reply proto.BecomePrimaryVmOwnerResponse
	err := client.RequestReply("Hypervisor.BecomePrimaryVmOwner", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmConsoleType(client srpc.ClientI, ipAddress net.IP,
	consoleType proto.ConsoleType) error {
	request := proto.ChangeVmConsoleTypeRequest{
		ConsoleType: consoleType,
		IpAddress:   ipAddress,
	}
	var reply proto.ChangeVmConsoleTypeResponse
	err := client.RequestReply("Hypervisor.ChangeVmConsoleType", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmCpuPriority(client srpc.ClientI,
	request proto.ChangeVmCpuPriorityRequest) error {
	var reply proto.ChangeVmCpuPriorityResponse
	err := client.RequestReply("Hypervisor.ChangeVmCpuPriority", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmDestroyProtection(client srpc.ClientI, ipAddress net.IP,
	destroyProtection bool) error {
	request := proto.ChangeVmDestroyProtectionRequest{
		DestroyProtection: destroyProtection,
		IpAddress:         ipAddress,
	}
	var reply proto.ChangeVmDestroyProtectionResponse
	err := client.RequestReply("Hypervisor.ChangeVmDestroyProtection",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmHostname(client srpc.ClientI, ipAddress net.IP,
	hostname string) error {
	request := proto.ChangeVmHostnameRequest{
		Hostname:  hostname,
		IpAddress: ipAddress,
	}
	var reply proto.ChangeVmHostnameResponse
	err := client.RequestReply("Hypervisor.ChangeVmHostname", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmMachineType(client srpc.ClientI, ipAddress net.IP,
	consoleType proto.MachineType) error {
	request := proto.ChangeVmMachineTypeRequest{
		MachineType: consoleType,
		IpAddress:   ipAddress,
	}
	var reply proto.ChangeVmMachineTypeResponse
	err := client.RequestReply("Hypervisor.ChangeVmMachineType", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmOwnerGroups(client srpc.ClientI, ipAddress net.IP,
	ownerGroups []string) error {
	request := proto.ChangeVmOwnerGroupsRequest{ipAddress, ownerGroups}
	var reply proto.ChangeVmOwnerGroupsResponse
	err := client.RequestReply("Hypervisor.ChangeVmOwnerGroups", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmOwnerUsers(client srpc.ClientI, ipAddress net.IP,
	ownerUsers []string) error {
	request := proto.ChangeVmOwnerUsersRequest{ipAddress, ownerUsers}
	var reply proto.ChangeVmOwnerUsersResponse
	err := client.RequestReply("Hypervisor.ChangeVmOwnerUsers", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmSize(client srpc.ClientI,
	request proto.ChangeVmSizeRequest) error {
	var reply proto.ChangeVmSizeResponse
	err := client.RequestReply("Hypervisor.ChangeVmSize", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmSubnet(client srpc.ClientI,
	request proto.ChangeVmSubnetRequest) (proto.ChangeVmSubnetResponse, error) {
	var reply proto.ChangeVmSubnetResponse
	err := client.RequestReply("Hypervisor.ChangeVmSubnet", request, &reply)
	if err != nil {
		return proto.ChangeVmSubnetResponse{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.ChangeVmSubnetResponse{}, err
	}
	return reply, nil
}

func changeVmTags(client srpc.ClientI, ipAddress net.IP, tgs tags.Tags) error {
	request := proto.ChangeVmTagsRequest{ipAddress, tgs}
	var reply proto.ChangeVmTagsResponse
	err := client.RequestReply("Hypervisor.ChangeVmTags", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmVolumeInterfaces(client srpc.ClientI, ipAddress net.IP,
	volumeInterfaces []proto.VolumeInterface) error {
	request := proto.ChangeVmVolumeInterfacesRequest{
		Interfaces: volumeInterfaces,
		IpAddress:  ipAddress,
	}
	var reply proto.ChangeVmVolumeInterfacesResponse
	err := client.RequestReply("Hypervisor.ChangeVmVolumeInterfaces", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmVolumeSize(client srpc.ClientI, ipAddress net.IP, index uint,
	size uint64) error {
	request := proto.ChangeVmVolumeSizeRequest{
		IpAddress:   ipAddress,
		VolumeIndex: index,
		VolumeSize:  size,
	}
	var reply proto.ChangeVmVolumeSizeResponse
	err := client.RequestReply("Hypervisor.ChangeVmVolumeSize", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func commitImportedVm(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.CommitImportedVmRequest{ipAddress}
	var reply proto.CommitImportedVmResponse
	err := client.RequestReply("Hypervisor.CommitImportedVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func connectToVmConsole(client srpc.ClientI, ipAddr net.IP,
	vncViewerCommand string, logger log.DebugLogger) error {
	serverConn, err := client.Call("Hypervisor.ConnectToVmConsole")
	if err != nil {
		return err
	}
	defer serverConn.Close()
	request := proto.ConnectToVmConsoleRequest{IpAddress: ipAddr}
	if err := serverConn.Encode(request); err != nil {
		return err
	}
	if err := serverConn.Flush(); err != nil {
		return err
	}
	var response proto.ConnectToVmConsoleResponse
	if err := serverConn.Decode(&response); err != nil {
		return err
	}
	if err := errors.New(response.Error); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return err
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	if vncViewerCommand == "" {
		logger.Printf("listening on port %s for VNC connection\n", port)
	} else {
		cmd := exec.Command(vncViewerCommand, "::"+port)
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return err
		}
	}
	clientConn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	listener.Close()
	var readErr error
	readFinished := false
	go func() { // Copy from server to client.
		_, readErr = io.Copy(clientConn, serverConn)
		readFinished = true
	}()
	// Copy from client to server.
	_, writeErr := io.Copy(bufwriter.NewAutoFlushWriter(serverConn), clientConn)
	if readFinished {
		return readErr
	}
	return writeErr
}

func createVm(client srpc.ClientI, request proto.CreateVmRequest,
	reply *proto.CreateVmResponse, logger log.DebugLogger) error {
	if conn, err := client.Call("Hypervisor.CreateVm"); err != nil {
		return err
	} else {
		defer conn.Close()
		if err := conn.Encode(request); err != nil {
			return err
		}
		if err := conn.Flush(); err != nil {
			return err
		}
		for {
			var response proto.CreateVmResponse
			if err := conn.Decode(&response); err != nil {
				return fmt.Errorf("error decoding: %s", err)
			}
			if response.Error != "" {
				return errors.New(response.Error)
			}
			if response.ProgressMessage != "" {
				logger.Debugln(0, response.ProgressMessage)
			}
			if response.Final {
				*reply = response
				return nil
			}
		}
	}
}

func deleteVmVolume(client srpc.ClientI, ipAddr net.IP, accessToken []byte,
	volumeIndex uint) error {
	request := proto.DeleteVmVolumeRequest{
		AccessToken: accessToken,
		IpAddress:   ipAddr,
		VolumeIndex: volumeIndex,
	}
	var reply proto.DeleteVmVolumeResponse
	err := client.RequestReply("Hypervisor.DeleteVmVolume", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func destroyVm(client srpc.ClientI, ipAddr net.IP, accessToken []byte) error {
	request := proto.DestroyVmRequest{
		AccessToken: accessToken,
		IpAddress:   ipAddr,
	}
	var reply proto.DestroyVmResponse
	err := client.RequestReply("Hypervisor.DestroyVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func discardVmAccessToken(client srpc.ClientI, ipAddress net.IP,
	token []byte) error {
	request := proto.DiscardVmAccessTokenRequest{
		AccessToken: token,
		IpAddress:   ipAddress}
	var reply proto.DiscardVmAccessTokenResponse
	err := client.RequestReply("Hypervisor.DiscardVmAccessToken", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func discardVmOldImage(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.DiscardVmOldImageRequest{ipAddress}
	var reply proto.DiscardVmOldImageResponse
	err := client.RequestReply("Hypervisor.DiscardVmOldImage", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func discardVmOldUserData(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.DiscardVmOldUserDataRequest{ipAddress}
	var reply proto.DiscardVmOldUserDataResponse
	err := client.RequestReply("Hypervisor.DiscardVmOldUserData", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func discardVmSnapshot(client srpc.ClientI, ipAddress net.IP,
	name string) error {
	request := proto.DiscardVmSnapshotRequest{
		IpAddress: ipAddress,
		Name:      name,
	}
	var reply proto.DiscardVmSnapshotResponse
	err := client.RequestReply("Hypervisor.DiscardVmSnapshot", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func exportLocalVm(client srpc.ClientI, ipAddr net.IP,
	verificationCookie []byte) (proto.ExportLocalVmInfo, error) {
	request := proto.ExportLocalVmRequest{
		IpAddress:          ipAddr,
		VerificationCookie: verificationCookie,
	}
	var reply proto.ExportLocalVmResponse
	err := client.RequestReply("Hypervisor.ExportLocalVm", request, &reply)
	if err != nil {
		return proto.ExportLocalVmInfo{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.ExportLocalVmInfo{}, err
	}
	return reply.VmInfo, nil
}

func getCapacity(client srpc.ClientI) (proto.GetCapacityResponse, error) {
	request := proto.GetCapacityRequest{}
	var reply proto.GetCapacityResponse
	err := client.RequestReply("Hypervisor.GetCapacity", request, &reply)
	if err != nil {
		return proto.GetCapacityResponse{}, err
	}
	return reply, nil
}

func getIdentityProvider(client srpc.ClientI) (string, error) {
	request := proto.GetIdentityProviderRequest{}
	var reply proto.GetIdentityProviderResponse
	err := client.RequestReply("Hypervisor.GetIdentityProvider",
		request, &reply)
	if err != nil {
		return "", err
	}
	if err := errors.New(reply.Error); err != nil {
		return "", err
	}
	return reply.BaseUrl, nil
}

func getPublicKey(client srpc.ClientI) ([]byte, error) {
	request := proto.GetPublicKeyRequest{}
	var reply proto.GetPublicKeyResponse
	err := client.RequestReply("Hypervisor.GetPublicKey", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.KeyPEM, nil
}

func getRootCookiePath(client srpc.ClientI) (string, error) {
	request := proto.GetRootCookiePathRequest{}
	var reply proto.GetRootCookiePathResponse
	err := client.RequestReply("Hypervisor.GetRootCookiePath", request, &reply)
	if err != nil {
		return "", err
	}
	if err := errors.New(reply.Error); err != nil {
		return "", err
	}
	return reply.Path, nil
}

func getVmAccessToken(client srpc.ClientI, ipAddress net.IP,
	lifetime time.Duration) ([]byte, error) {
	request := proto.GetVmAccessTokenRequest{ipAddress, lifetime}
	var reply proto.GetVmAccessTokenResponse
	err := client.RequestReply("Hypervisor.GetVmAccessToken", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.Token, nil
}

func getVmCreateRequest(client srpc.ClientI, ipAddr net.IP) (
	proto.CreateVmRequest, error) {
	request := proto.GetVmCreateRequestRequest{IpAddress: ipAddr}
	var reply proto.GetVmCreateRequestResponse
	err := client.RequestReply("Hypervisor.GetVmCreateRequest", request, &reply)
	if err != nil {
		return proto.CreateVmRequest{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.CreateVmRequest{}, err
	}
	return reply.CreateVmRequest, nil
}

func getVmInfo(client srpc.ClientI, ipAddr net.IP) (proto.VmInfo, error) {
	request := proto.GetVmInfoRequest{IpAddress: ipAddr}
	var reply proto.GetVmInfoResponse
	err := client.RequestReply("Hypervisor.GetVmInfo", request, &reply)
	if err != nil {
		return proto.VmInfo{}, err
	}
	if err := errors.New(reply.Error); err != nil {
		return proto.VmInfo{}, err
	}
	return reply.VmInfo, nil
}

func getVmInfos(client srpc.ClientI,
	request proto.GetVmInfosRequest) ([]proto.VmInfo, error) {
	var reply proto.GetVmInfosResponse
	err := client.RequestReply("Hypervisor.GetVmInfos", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.VmInfos, nil
}

func getVmLastPatchLog(client srpc.ClientI, ipAddr net.IP) (
	[]byte, time.Time, error) {
	conn, err := client.Call("Hypervisor.GetVmLastPatchLog")
	if err != nil {
		return nil, time.Time{}, err
	}
	defer conn.Close()
	request := proto.GetVmLastPatchLogRequest{IpAddress: ipAddr}
	if err := conn.Encode(request); err != nil {
		return nil, time.Time{}, err
	}
	if err := conn.Flush(); err != nil {
		return nil, time.Time{}, err
	}
	var response proto.GetVmLastPatchLogResponse
	if err := conn.Decode(&response); err != nil {
		return nil, time.Time{}, err
	}
	if err := errors.New(response.Error); err != nil {
		return nil, time.Time{}, err
	}
	buffer := &bytes.Buffer{}
	if _, err := io.CopyN(buffer, conn, int64(response.Length)); err != nil {
		return nil, time.Time{}, err
	}
	return buffer.Bytes(), response.PatchTime, nil
}

func holdLock(client srpc.ClientI, timeout time.Duration,
	writeLock bool) error {
	request := proto.HoldLockRequest{timeout, writeLock}
	var reply proto.HoldLockResponse
	err := client.RequestReply("Hypervisor.HoldLock", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func holdVmLock(client srpc.ClientI, ipAddr net.IP, timeout time.Duration,
	writeLock bool) error {
	request := proto.HoldVmLockRequest{ipAddr, timeout, writeLock}
	var reply proto.HoldVmLockResponse
	err := client.RequestReply("Hypervisor.HoldVmLock", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func importLocalVm(client srpc.ClientI,
	request proto.ImportLocalVmRequest) error {
	var reply proto.ImportLocalVmResponse
	err := client.RequestReply("Hypervisor.ImportLocalVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func listSubnets(client srpc.ClientI, doSort bool) ([]proto.Subnet, error) {
	request := proto.ListSubnetsRequest{Sort: doSort}
	var reply proto.ListSubnetsResponse
	err := client.RequestReply("Hypervisor.ListSubnets", request, &reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	return reply.Subnets, nil
}

func listVMs(client srpc.ClientI,
	request proto.ListVMsRequest) ([]net.IP, error) {
	var reply proto.ListVMsResponse
	err := client.RequestReply("Hypervisor.ListVMs", request, &reply)
	if err != nil {
		return nil, err
	}
	return reply.IpAddresses, nil
}

func listVolumeDirectories(client srpc.ClientI, doSort bool) ([]string, error) {
	var request proto.ListVolumeDirectoriesRequest
	var reply proto.ListVolumeDirectoriesResponse
	err := client.RequestReply("Hypervisor.ListVolumeDirectories", request,
		&reply)
	if err != nil {
		return nil, err
	}
	if err := errors.New(reply.Error); err != nil {
		return nil, err
	}
	if doSort {
		sort.Strings(reply.Directories)
	}
	return reply.Directories, nil
}

func migrateVm(client srpc.ClientI, request proto.MigrateVmRequest,
	commitFunc func() bool, logger log.DebugLogger) error {
	conn, err := client.Call("Hypervisor.MigrateVm")
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	for {
		var reply proto.MigrateVmResponse
		if err := conn.Decode(&reply); err != nil {
			return err
		}
		if reply.Error != "" {
			return errors.New(reply.Error)
		}
		if reply.ProgressMessage != "" {
			logger.Debugln(0, reply.ProgressMessage)
		}
		if reply.RequestCommit {
			commitResponse := proto.MigrateVmResponseResponse{commitFunc()}
			if err := conn.Encode(commitResponse); err != nil {
				return err
			}
			if err := conn.Flush(); err != nil {
				return err
			}
		}
		if reply.Final {
			break
		}
	}
	return nil
}

func powerOff(client srpc.ClientI, stopVMs bool) error {
	request := proto.PowerOffRequest{StopVMs: stopVMs}
	var reply proto.PowerOffResponse
	err := client.RequestReply("Hypervisor.PowerOff", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func prepareVmForMigration(client srpc.ClientI, ipAddr net.IP,
	accessToken []byte, enable bool) error {
	request := proto.PrepareVmForMigrationRequest{
		AccessToken: accessToken,
		Enable:      enable,
		IpAddress:   ipAddr,
	}
	var reply proto.PrepareVmForMigrationResponse
	err := client.RequestReply("Hypervisor.PrepareVmForMigration",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func probeVmPort(client srpc.ClientI, request proto.ProbeVmPortRequest) (
	proto.ProbeVmPortResponse, error) {
	var response proto.ProbeVmPortResponse
	err := client.RequestReply("Hypervisor.ProbeVmPort", request,
		&response)
	if err != nil {
		return response, err
	}
	return response, errors.New(response.Error)
}

func rebootVm(client srpc.ClientI, ipAddress net.IP,
	dhcpTimeout time.Duration) (bool, error) {
	request := proto.RebootVmRequest{
		DhcpTimeout: dhcpTimeout,
		IpAddress:   ipAddress,
	}
	var reply proto.RebootVmResponse
	err := client.RequestReply("Hypervisor.RebootVm", request, &reply)
	if err != nil {
		return false, err
	}
	if err := errors.New(reply.Error); err != nil {
		return false, err
	}
	return reply.DhcpTimedOut, nil
}

func registerExternalLeases(client srpc.ClientI, addressList proto.AddressList,
	hostnames []string) error {
	request := proto.RegisterExternalLeasesRequest{
		Addresses: addressList,
		Hostnames: hostnames,
	}
	var reply proto.RegisterExternalLeasesResponse
	err := client.RequestReply("Hypervisor.RegisterExternalLeases",
		request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func reorderVmVolumes(client srpc.ClientI, ipAddr net.IP, accessToken []byte,
	volumeIndices []uint) error {
	request := proto.ReorderVmVolumesRequest{
		IpAddress:     ipAddr,
		VolumeIndices: volumeIndices,
	}
	var reply proto.ReorderVmVolumesResponse
	err := client.RequestReply("Hypervisor.ReorderVmVolumes", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func replaceVmCredentials(client srpc.ClientI,
	request proto.ReplaceVmCredentialsRequest) error {
	var response proto.ReplaceVmCredentialsResponse
	err := client.RequestReply("Hypervisor.ReplaceVmCredentials", request,
		&response)
	if err != nil {
		return err
	}
	return errors.New(response.Error)
}

func replaceVmIdentity(client srpc.ClientI,
	request proto.ReplaceVmIdentityRequest) error {
	var response proto.ReplaceVmIdentityResponse
	err := client.RequestReply("Hypervisor.ReplaceVmIdentity", request,
		&response)
	if err != nil {
		return err
	}
	return errors.New(response.Error)
}

func restoreVmFromSnapshot(client srpc.ClientI,
	request proto.RestoreVmFromSnapshotRequest) error {
	var response proto.RestoreVmFromSnapshotResponse
	err := client.RequestReply("Hypervisor.RestoreVmFromSnapshot", request,
		&response)
	if err != nil {
		return err
	}
	return errors.New(response.Error)
}

func restoreVmImage(client srpc.ClientI,
	request proto.RestoreVmImageRequest) error {
	var response proto.RestoreVmImageResponse
	err := client.RequestReply("Hypervisor.RestoreVmImage", request,
		&response)
	if err != nil {
		return err
	}
	return errors.New(response.Error)
}

func restoreVmUserData(client srpc.ClientI, ipAddress net.IP) error {
	request := proto.RestoreVmUserDataRequest{ipAddress}
	var reply proto.RestoreVmUserDataResponse
	err := client.RequestReply("Hypervisor.RestoreVmUserData", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func scanVmRoot(client srpc.ClientI, ipAddr net.IP,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	request := proto.ScanVmRootRequest{IpAddress: ipAddr, Filter: scanFilter}
	var reply proto.ScanVmRootResponse
	err := client.RequestReply("Hypervisor.ScanVmRoot", request, &reply)
	if err != nil {
		return nil, err
	}
	return reply.FileSystem, errors.New(reply.Error)
}

func setDisabledState(client srpc.ClientI, disable bool) error {
	request := proto.SetDisabledStateRequest{Disable: disable}
	var reply proto.SetDisabledStateResponse
	err := client.RequestReply("Hypervisor.SetDisabledState", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func snapshotVm(client srpc.ClientI,
	request proto.SnapshotVmRequest) error {
	var reply proto.SnapshotVmResponse
	err := client.RequestReply("Hypervisor.SnapshotVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func startVm(client srpc.ClientI, ipAddr net.IP, accessToken []byte,
	dhcpTimeout time.Duration) (bool, error) {
	request := proto.StartVmRequest{
		AccessToken: accessToken,
		DhcpTimeout: dhcpTimeout,
		IpAddress:   ipAddr,
	}
	var reply proto.StartVmResponse
	err := client.RequestReply("Hypervisor.StartVm", request, &reply)
	if err != nil {
		return false, err
	}
	return reply.DhcpTimedOut, errors.New(reply.Error)
}

func stopVm(client srpc.ClientI, ipAddr net.IP, accessToken []byte) error {
	request := proto.StopVmRequest{
		AccessToken: accessToken,
		IpAddress:   ipAddr,
	}
	var reply proto.StopVmResponse
	err := client.RequestReply("Hypervisor.StopVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}
