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
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func acknowledgeVm(client *srpc.Client, ipAddress net.IP) error {
	request := proto.AcknowledgeVmRequest{ipAddress}
	var reply proto.AcknowledgeVmResponse
	return client.RequestReply("Hypervisor.AcknowledgeVm", request, &reply)
}

func addVmVolumes(client *srpc.Client, ipAddress net.IP, sizes []uint64) error {
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

func changeVmCpuPriority(client *srpc.Client, ipAddress net.IP,
	request proto.ChangeVmCpuPriorityRequest) error {
	var reply proto.ChangeVmCpuPriorityResponse
	err := client.RequestReply("Hypervisor.ChangeVmCpuPriority", request,
		&reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmSize(client *srpc.Client,
	request proto.ChangeVmSizeRequest) error {
	var reply proto.ChangeVmSizeResponse
	err := client.RequestReply("Hypervisor.ChangeVmSize", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func changeVmVolumeSize(client *srpc.Client, ipAddress net.IP, index uint,
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

func connectToVmConsole(client *srpc.Client, ipAddr net.IP,
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

func createVm(client *srpc.Client, request proto.CreateVmRequest,
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

func deleteVmVolume(client *srpc.Client, ipAddr net.IP, accessToken []byte,
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

func destroyVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
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

func exportLocalVm(client *srpc.Client, ipAddr net.IP,
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

func getCapacity(client *srpc.Client) (proto.GetCapacityResponse, error) {
	request := proto.GetCapacityRequest{}
	var reply proto.GetCapacityResponse
	err := client.RequestReply("Hypervisor.GetCapacity", request, &reply)
	if err != nil {
		return proto.GetCapacityResponse{}, err
	}
	return reply, nil
}

func getRootCookiePath(client *srpc.Client) (string, error) {
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

func getVmInfo(client *srpc.Client, ipAddr net.IP) (proto.VmInfo, error) {
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

func getVmInfos(client *srpc.Client,
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

func getVmLastPatchLog(client *srpc.Client, ipAddr net.IP) (
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

func holdLock(client *srpc.Client, timeout time.Duration,
	writeLock bool) error {
	request := proto.HoldLockRequest{timeout, writeLock}
	var reply proto.HoldLockResponse
	err := client.RequestReply("Hypervisor.HoldLock", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func holdVmLock(client *srpc.Client, ipAddr net.IP, timeout time.Duration,
	writeLock bool) error {
	request := proto.HoldVmLockRequest{ipAddr, timeout, writeLock}
	var reply proto.HoldVmLockResponse
	err := client.RequestReply("Hypervisor.HoldVmLock", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func listSubnets(client *srpc.Client, doSort bool) ([]proto.Subnet, error) {
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

func listVMs(client *srpc.Client,
	request proto.ListVMsRequest) ([]net.IP, error) {
	var reply proto.ListVMsResponse
	err := client.RequestReply("Hypervisor.ListVMs", request, &reply)
	if err != nil {
		return nil, err
	}
	return reply.IpAddresses, nil
}

func listVolumeDirectories(client *srpc.Client, doSort bool) ([]string, error) {
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

func powerOff(client *srpc.Client, stopVMs bool) error {
	request := proto.PowerOffRequest{StopVMs: stopVMs}
	var reply proto.PowerOffResponse
	err := client.RequestReply("Hypervisor.PowerOff", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func prepareVmForMigration(client *srpc.Client, ipAddr net.IP,
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

func registerExternalLeases(client *srpc.Client, addressList proto.AddressList,
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

func reorderVmVolumes(client *srpc.Client, ipAddr net.IP, accessToken []byte,
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

func scanVmRoot(client *srpc.Client, ipAddr net.IP,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	request := proto.ScanVmRootRequest{IpAddress: ipAddr, Filter: scanFilter}
	var reply proto.ScanVmRootResponse
	err := client.RequestReply("Hypervisor.ScanVmRoot", request, &reply)
	if err != nil {
		return nil, err
	}
	return reply.FileSystem, errors.New(reply.Error)
}

func setDisabledState(client *srpc.Client, disable bool) error {
	request := proto.SetDisabledStateRequest{Disable: disable}
	var reply proto.SetDisabledStateResponse
	err := client.RequestReply("Hypervisor.SetDisabledState", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func startVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
	request := proto.StartVmRequest{
		AccessToken: accessToken,
		IpAddress:   ipAddr,
	}
	var reply proto.StartVmResponse
	err := client.RequestReply("Hypervisor.StartVm", request, &reply)
	if err != nil {
		return err
	}
	return errors.New(reply.Error)
}

func stopVm(client *srpc.Client, ipAddr net.IP, accessToken []byte) error {
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
