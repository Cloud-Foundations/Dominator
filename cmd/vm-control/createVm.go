package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/images/qcow2"
	"github.com/Cloud-Foundations/Dominator/lib/images/virtualbox"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var sysfsDirectory = "/sys/block"

type createVmRequest struct {
	hyper_proto.CreateVmRequest
	imageReader io.ReadCloser
}

type volumeInitParams struct {
	hyper_proto.VolumeInitialisationInfo
	MountPoint string
}

type wrappedReadCloser struct {
	real io.Closer
	wrap io.Reader
}

func init() {
	rand.Seed(time.Now().Unix() + time.Now().UnixNano())
}

func approximateImageUsage() (uint64, error) {
	size, err := getImageUsage()
	if err != nil {
		return 0, err
	}
	size += size >> 3 // 12% extra for good luck.
	size += uint64(minFreeBytes)
	imageUnits := size >> *roundupPower
	if imageUnits<<*roundupPower < size {
		imageUnits++
	}
	return imageUnits << *roundupPower, nil
}

// approximateVolumesForCreateRequest will make a modified copy of a VmInfo,
// filling in approximate volume sizes. This may be used for placement
// decisions.
func approximateVolumesForCreateRequest(
	vmInfo hyper_proto.VmInfo) (*hyper_proto.VmInfo, error) {
	vmInfo.Volumes = make([]hyper_proto.Volume, 1, len(secondaryVolumeSizes)+1)
	for _, size := range secondaryVolumeSizes {
		vmInfo.Volumes = append(vmInfo.Volumes, hyper_proto.Volume{
			Size: uint64(size),
		})
	}
	if *imageName != "" {
		imageSize, err := approximateImageUsage()
		if err != nil {
			return nil, err
		}
		vmInfo.Volumes[0] = hyper_proto.Volume{Size: imageSize}
		return &vmInfo, nil
	}
	if *imageFile != "" {
		fi, err := os.Stat(*imageFile)
		if err != nil {
			return nil, err
		}
		vmInfo.Volumes[0].Size = uint64(fi.Size())
		if volumeFormat == hyper_proto.VolumeFormatQCOW2 {
			qcow2Header, err := qcow2.ReadHeaderFromFile(*imageFile)
			if err != nil {
				return nil, err
			}
			vmInfo.Volumes[0].VirtualSize = qcow2Header.Size
		}
		return &vmInfo, nil
	}
	if *imageURL != "" {
		httpResponse, err := http.Get(*imageURL)
		if err != nil {
			return nil, err
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			return nil, errors.New(httpResponse.Status)
		}
		if httpResponse.ContentLength < 0 {
			return nil, errors.New("ContentLength from: " + *imageURL)
		}
		vmInfo.Volumes[0].Size = uint64(httpResponse.ContentLength)
		if volumeFormat == hyper_proto.VolumeFormatQCOW2 {
			qcow2Header, err := qcow2.ReadHeader(httpResponse.Body)
			if err != nil {
				return nil, err
			}
			vmInfo.Volumes[0].VirtualSize = qcow2Header.Size
		}
		return &vmInfo, nil
	}
	return &vmInfo, nil
}

func createVmSubcommand(args []string, logger log.DebugLogger) error {
	if err := createVm(logger); err != nil {
		return fmt.Errorf("error creating VM: %s", err)
	}
	return nil
}

func callCreateVm(client *srpc.Client, request hyper_proto.CreateVmRequest,
	reply *hyper_proto.CreateVmResponse, imageReader, userDataReader io.Reader,
	imageSize, userDataSize int64, logger log.DebugLogger) error {
	conn, err := hyperclient.OpenCreateVmConn(client, request)
	if err != nil {
		return err
	}
	doClose := true
	defer func() {
		if doClose {
			conn.Close()
		}
	}()
	// Stream any required data.
	if imageReader != nil {
		logger.Debugln(0, "uploading image")
		startTime := time.Now()
		if nCopied, err := io.CopyN(conn, imageReader, imageSize); err != nil {
			return fmt.Errorf("error uploading image: %s got %d of %d bytes",
				err, nCopied, imageSize)
		} else {
			duration := time.Since(startTime)
			speed := uint64(float64(nCopied) / duration.Seconds())
			logger.Debugf(0, "uploaded image in %s (%s/s)\n",
				format.Duration(duration), format.FormatBytes(speed))
		}
	}
	if userDataReader != nil {
		logger.Debugln(0, "uploading user data")
		nCopied, err := io.CopyN(conn, userDataReader, userDataSize)
		if err != nil {
			return fmt.Errorf(
				"error uploading user data: %s got %d of %d bytes",
				err, nCopied, userDataSize)
		}
	}
	response, err := hyperclient.ProcessCreateVmResponses(conn, logger)
	if err != nil {
		return err
	}
	*reply = response
	doClose = false
	return conn.Close()
}

func checkTags(logger log.DebugLogger) {
	imageName := vmTags["RequiredImage"]
	if imageName == "" {
		return
	}
	client, err := getImageServerClient()
	if err != nil {
		logger.Println(err)
		return
	}
	if client == nil {
		return
	}
	expiresAt, err := imgclient.GetImageExpiration(client, imageName)
	if err != nil {
		logger.Println(err)
		return
	}
	if expiresAt.IsZero() {
		return
	}
	logger.Printf("WARNING: image: %s expires at: %s (in %s)\n",
		imageName,
		expiresAt.Format(format.TimeFormatSeconds),
		format.Duration(time.Until(expiresAt)))
}

func createVm(logger log.DebugLogger) error {
	request, err := makeVmCreateRequest(logger)
	if err != nil {
		return err
	}
	tmpVmInfo, err := approximateVolumesForCreateRequest(request.VmInfo)
	if err != nil {
		return err
	}
	if hypervisor, err := getHypervisorAddress(*tmpVmInfo, logger); err != nil {
		return err
	} else {
		logger.Debugf(0, "creating VM on %s\n", hypervisor)
		return createVmOnHypervisor(hypervisor, *request, logger)
	}
}

func createVmInfoFromFlags() (*hyper_proto.VmInfo, error) {
	var networkEntries []hyper_proto.NetworkEntry
	if len(numNetworkQueues) > 0 {
		for _, numQueues := range numNetworkQueues {
			networkEntries = append(networkEntries,
				hyper_proto.NetworkEntry{NumQueues: numQueues})
		}
	}
	var volumes []hyper_proto.Volume
	var volumeInterface hyper_proto.VolumeInterface
	if len(volumeInterfaces) > 0 {
		volumeInterface = volumeInterfaces[0]
	}
	var volumeType hyper_proto.VolumeType
	if len(volumeTypes) > 0 {
		volumeType = volumeTypes[0]
	}
	if volumeFormat != hyper_proto.VolumeFormatRaw ||
		volumeInterface != hyper_proto.VolumeInterfaceVirtIO ||
		volumeType != hyper_proto.VolumeTypePersistent {
		// If any provided, set for root volume. Secondaries are done later.
		volumes = append(volumes, hyper_proto.Volume{
			Format:    volumeFormat,
			Interface: volumeInterface,
			Type:      volumeType,
		})
	}
	vmInfo := hyper_proto.VmInfo{
		ConsoleType:        consoleType,
		CpuPriority:        *cpuPriority,
		DestroyOnPowerdown: *destroyOnPowerdown,
		DestroyProtection:  *destroyProtection,
		DisableVirtIO:      *disableVirtIO,
		ExtraKernelOptions: *extraKernelOptions,
		FirmwareType:       firmwareType,
		Hostname:           *vmHostname,
		MachineType:        machineType,
		MemoryInMiB:        uint64(memory >> 20),
		MilliCPUs:          *milliCPUs,
		NetworkEntries:     networkEntries,
		OwnerGroups:        ownerGroups,
		OwnerUsers:         ownerUsers,
		Tags:               vmTags,
		SecondarySubnetIDs: secondarySubnetIDs,
		SpreadVolumes:      *spreadVolumes,
		SubnetId:           *subnetId,
		VirtualCPUs:        *virtualCPUs,
		Volumes:            volumes,
		WatchdogAction:     watchdogAction,
		WatchdogModel:      watchdogModel,
	}
	if len(requestIPs) > 0 && requestIPs[0] != "" {
		ipAddr := net.ParseIP(requestIPs[0])
		if ipAddr == nil {
			return nil, fmt.Errorf("invalid IP address: %s", requestIPs[0])
		}
		vmInfo.Address.IpAddress = ipAddr
	}
	if len(requestIPs) > 1 && len(secondarySubnetIDs) > 0 {
		vmInfo.SecondaryAddresses = make([]hyper_proto.Address,
			len(secondarySubnetIDs))
		for index, addr := range requestIPs[1:] {
			if addr == "" {
				continue
			}
			ipAddr := net.ParseIP(addr)
			if ipAddr == nil {
				return nil, fmt.Errorf("invalid IP address: %s", requestIPs[0])
			}
			vmInfo.SecondaryAddresses[index] = hyper_proto.Address{
				IpAddress: ipAddr}
		}
	}
	return &vmInfo, nil
}

func createVmOnHypervisor(hypervisor string,
	request createVmRequest, logger log.DebugLogger) error {
	if request.imageReader != nil {
		defer request.imageReader.Close()
	}
	var userDataReader io.Reader
	if *userDataFile != "" {
		file, size, err := getReader(*userDataFile)
		if err != nil {
			return err
		} else {
			defer file.Close()
			request.UserDataSize = uint64(size)
			userDataReader = file
		}
	}
	client, err := dialHypervisor(hypervisor)
	if err != nil {
		return err
	}
	defer client.Close()
	var reply hyper_proto.CreateVmResponse
	err = callCreateVm(client, request.CreateVmRequest, &reply,
		request.imageReader, userDataReader, int64(request.ImageDataSize),
		int64(request.UserDataSize), logger)
	if err != nil {
		return err
	}
	if err := hyperclient.AcknowledgeVm(client, reply.IpAddress); err != nil {
		return fmt.Errorf("error acknowledging VM: %s", err)
	}
	if *identityName != "" {
		err := setupVmWithIdentity(client, hypervisor, reply.IpAddress, logger)
		if err != nil {
			e := hyperclient.DestroyVm(client, reply.IpAddress, nil)
			if e != nil {
				logger.Println(e)
			}
			return err
		}
	}
	fmt.Println(reply.IpAddress)
	if *doNotStart {
		return nil
	}
	if reply.DhcpTimedOut {
		return errors.New("DHCP ACK timed out")
	}
	if *dhcpTimeout > 0 {
		logger.Debugln(0, "Received DHCP ACK")
	}
	return maybeWatchVm(client, hypervisor, reply.IpAddress, logger)
}

func getReader(filename string) (io.ReadCloser, int64, error) {
	if file, err := os.Open(filename); err != nil {
		return nil, -1, err
	} else if filepath.Ext(filename) == ".vdi" {
		vdi, err := virtualbox.NewReader(file)
		if err != nil {
			file.Close()
			return nil, -1, err
		}
		return &wrappedReadCloser{real: file, wrap: vdi}, int64(vdi.Size), nil
	} else {
		fi, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, -1, err
		}
		switch fi.Mode() & os.ModeType {
		case 0:
			return file, fi.Size(), nil
		case os.ModeDevice:
			if size, err := readBlockDeviceSize(filename); err != nil {
				file.Close()
				return nil, -1, err
			} else {
				return file, size, nil
			}
		default:
			file.Close()
			return nil, -1, errors.New("unsupported file type")
		}
	}
}

func getImageUsage() (uint64, error) {
	client, err := getImageServerClient()
	if err != nil {
		logger.Printf(
			"error connecting to imageserver: %s, guessing image size\n", err)
		return 2 << 30, nil
	}
	if client == nil {
		logger.Printf("no imageserver specified, guessing image size\n")
		return 2 << 30, nil
	}
	var name string
	if isDir, err := imgclient.CheckDirectory(client, *imageName); err != nil {
		return 0, err
	} else if isDir {
		name, err = imgclient.FindLatestImage(client, *imageName, false)
		if err != nil {
			return 0, err
		}
		if name == "" {
			return 0, errors.New("no images in directory: " + *imageName)
		}
	} else {
		name = *imageName
	}
	usage, exists, err := imgclient.GetImageUsageEstimate(client, name)
	if err != nil {
		logger.Printf(
			"error getting usage for image: %s, guessing image size\n", err)
		return 2 << 30, nil
	}
	if exists {
		return usage, nil
	}
	logger.Printf("image: %s not found, guessing image size\n", name)
	return 2 << 30, nil
}

func loadOverlayFiles() (map[string][]byte, error) {
	if *overlayDirectory == "" {
		return nil, nil
	}
	return fsutil.ReadFileTree(*overlayDirectory, *overlayPrefix)
}

func makeVmCreateRequest(logger log.DebugLogger) (*createVmRequest, error) {
	if *requestFile != "" {
		var request createVmRequest
		if err := json.ReadFromFile(*requestFile, &request); err != nil {
			return nil, err
		}
		return &request, nil
	}
	if *vmHostname == "" {
		if name := vmTags["Name"]; name == "" {
			return nil, errors.New("no hostname specified")
		} else {
			*vmHostname = name
		}
	} else {
		if name := vmTags["Name"]; name == "" {
			if vmTags == nil {
				vmTags = make(tags.Tags)
			}
			vmTags["Name"] = *vmHostname
		}
	}
	checkTags(logger)
	vmInfo, err := createVmInfoFromFlags()
	if err != nil {
		return nil, err
	}
	request := createVmRequest{
		CreateVmRequest: hyper_proto.CreateVmRequest{
			DhcpTimeout:      *dhcpTimeout,
			DoNotStart:       *doNotStart,
			EnableNetboot:    *enableNetboot,
			MinimumFreeBytes: uint64(minFreeBytes),
			RoundupPower:     *roundupPower,
			SkipMemoryCheck:  *skipMemoryCheck,
			StorageIndices:   storageIndices,
			VmInfo:           *vmInfo,
		}}
	if request.VmInfo.MemoryInMiB < 1 {
		request.VmInfo.MemoryInMiB = 1024
	}
	if request.VmInfo.MilliCPUs < 1 {
		request.VmInfo.MilliCPUs = 250
	}
	minimumCPUs := request.VmInfo.MilliCPUs / 1000
	if request.VmInfo.VirtualCPUs > 0 &&
		request.VmInfo.VirtualCPUs < minimumCPUs {
		return nil, fmt.Errorf("vCPUs must be at least %d", minimumCPUs)
	}
	secondaryFstab := &bytes.Buffer{}
	var vinitParams []volumeInitParams
	if *secondaryVolumesInitParams == "" {
		vinitParams = makeVolumeInitParams(uint(len(secondaryVolumeSizes)))
	} else {
		err := json.ReadFromFile(*secondaryVolumesInitParams, &vinitParams)
		if err != nil {
			return nil, err
		}
	}
	if err := updateVolumeInitParams(vinitParams); err != nil {
		return nil, err
	}
	for index, size := range secondaryVolumeSizes {
		volume := hyper_proto.Volume{Size: uint64(size)}
		if index+1 < len(volumeInterfaces) {
			volume.Interface = volumeInterfaces[index+1]
		}
		if index+1 < len(volumeTypes) {
			volume.Type = volumeTypes[index+1]
		}
		request.SecondaryVolumes = append(request.SecondaryVolumes, volume)
		if *initialiseSecondaryVolumes &&
			index < len(vinitParams) {
			vinit := vinitParams[index]
			if vinit.Label == "" {
				return nil, fmt.Errorf("VolumeInit[%d] missing Label", index)
			}
			if vinit.MountPoint == "" {
				return nil,
					fmt.Errorf("VolumeInit[%d] missing MountPoint", index)
			}
			request.OverlayDirectories = append(request.OverlayDirectories,
				vinit.MountPoint)
			request.SecondaryVolumesInit = append(request.SecondaryVolumesInit,
				vinit.VolumeInitialisationInfo)
			util.WriteFstabEntry(secondaryFstab, "LABEL="+vinit.Label,
				vinit.MountPoint, "ext4", "discard", 0, 2)
		}
	}
	if *identityName != "" {
		if *identityCertFile != "" {
			return nil, errors.New(
				"must not specify identityCertFile when specifying identityName")
		}
		if *identityKeyFile != "" {
			return nil, errors.New(
				"must not specify identityKeyFile when specifying identityName")
		}
		request.DoNotStart = true
	} else if *identityCertFile != "" && *identityKeyFile != "" {
		identityCert, err := ioutil.ReadFile(*identityCertFile)
		if err != nil {
			return nil, err
		}
		identityKey, err := ioutil.ReadFile(*identityKeyFile)
		if err != nil {
			return nil, err
		}
		request.IdentityCertificate = identityCert
		request.IdentityKey = identityKey
	}
	if *imageName != "" {
		request.ImageName = *imageName
		request.ImageTimeout = *imageTimeout
		request.SkipBootloader = *skipBootloader
		if overlayFiles, err := loadOverlayFiles(); err != nil {
			return nil, err
		} else {
			request.OverlayFiles = overlayFiles
		}
		secondaryFstab.Write(request.OverlayFiles["/etc/fstab"])
		if secondaryFstab.Len() > 0 {
			if request.OverlayFiles == nil {
				request.OverlayFiles = make(map[string][]byte)
			}
			request.OverlayFiles["/etc/fstab"] = secondaryFstab.Bytes()
		}
	} else if *imageURL != "" {
		request.ImageURL = *imageURL
	} else if *imageFile != "" {
		file, size, err := getReader(*imageFile)
		if err != nil {
			return nil, err
		} else {
			request.ImageDataSize = uint64(size)
			request.imageReader = file
		}
	} else {
		return nil, errors.New("no image specified")
	}
	return &request, nil
}

func makeVolumeInitParams(numVolumes uint) []volumeInitParams {
	vinitParams := make([]volumeInitParams, numVolumes)
	for index := 0; index < int(numVolumes); index++ {
		label := fmt.Sprintf("/data/%d", index)
		vinitParams[index].Label = label
		vinitParams[index].MountPoint = label
	}
	return vinitParams
}

func readBlockDeviceSize(filename string) (int64, error) {
	if strings.HasPrefix(filename, "/dev/") {
		filename = filename[5:]
	}
	deviceBlocks, err := readSysfsInt64(
		filepath.Join(sysfsDirectory, filename, "size"))
	if err != nil {
		return 0, err
	}
	return deviceBlocks * 512, nil
}

func readSysfsInt64(filename string) (int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var value int64
	nScanned, err := fmt.Fscanf(file, "%d", &value)
	if err != nil {
		return 0, err
	}
	if nScanned < 1 {
		return 0, fmt.Errorf("only read %d values from: %s", nScanned, filename)
	}
	return value, nil
}

func setupVmWithIdentity(client *srpc.Client, hypervisorAddress string,
	vmIP net.IP, logger log.DebugLogger) error {
	err := replaceVmIdentityOnConnectedHypervisor(client, hypervisorAddress,
		vmIP, logger)
	if err != nil {
		return err
	}
	if *doNotStart {
		return nil
	}
	return hyperclient.StartVm(client, vmIP, nil)
}

func updateVolumeInitParams(vinitParams []volumeInitParams) error {
	filenames := make([]filesystem.Filename, 0, len(vinitParams))
	allSpecified := true
	for _, vinit := range vinitParams {
		if vinit.RootGroupId == 0 && vinit.RootUserId == 0 {
			allSpecified = false
		}
		filenames = append(filenames, filesystem.Filename(vinit.MountPoint))
	}
	if allSpecified {
		return nil
	}
	client, err := getImageServerClient()
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}
	// TODO(rgooch): pass this in somehow to reduce duplication.
	var name string
	if isDir, err := imgclient.CheckDirectory(client, *imageName); err != nil {
		return err
	} else if isDir {
		name, err = imgclient.FindLatestImage(client, *imageName, false)
		if err != nil {
			return err
		}
		if name == "" {
			return errors.New("no images in directory: " + *imageName)
		}
	} else {
		name = *imageName
	}
	response, err := imgclient.GetImageInodes(client, name, filenames)
	if err != nil {
		logger.Println(err)
		return nil // Might not be supported, so don't fail.
	}
	if !response.ImageExists {
		return nil
	}
	for index, vinit := range vinitParams {
		if vinit.RootGroupId != 0 || vinit.RootUserId != 0 {
			continue // User-specified: leave it alone.
		}
		if inum, ok := response.InodeNumbers[filenames[index]]; !ok {
			continue
		} else if inode, ok := response.Inodes[inum]; !ok {
			continue
		} else {
			vinit.RootGroupId = hyper_proto.GroupId(inode.GetGid())
			vinit.RootUserId = hyper_proto.UserId(inode.GetUid())
			vinitParams[index] = vinit
		}
	}
	return nil
}

func (r *wrappedReadCloser) Close() error {
	return r.real.Close()
}

func (r *wrappedReadCloser) Read(p []byte) (n int, err error) {
	return r.wrap.Read(p)
}
