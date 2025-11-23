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

// approximateVolumesForCreateRequest will make a modified copy of a VmInfo,
// filling in approximate volume sizes. This may be used for placement
// decisions.
func approximateVolumesForCreateRequest(
	vmInfo hyper_proto.VmInfo) (*hyper_proto.VmInfo, error) {
	vmInfo.Volumes = make([]hyper_proto.Volume, 1, len(secondaryVolumeSizes)+1)
	vmInfo.Volumes[0] = hyper_proto.Volume{Size: uint64(minFreeBytes) + 2<<30}
	for _, size := range secondaryVolumeSizes {
		vmInfo.Volumes = append(vmInfo.Volumes, hyper_proto.Volume{
			Size: uint64(size),
		})
	}
	if *imageName != "" {
		return &vmInfo, nil
	}
	if *imageFile != "" && volumeFormat == hyper_proto.VolumeFormatQCOW2 {
		qcow2Header, err := qcow2.ReadHeaderFromFile(*imageFile)
		if err != nil {
			return nil, err
		}
		vmInfo.Volumes[0].VirtualSize = qcow2Header.Size
		return &vmInfo, nil
	}
	if *imageURL != "" && volumeFormat == hyper_proto.VolumeFormatQCOW2 {
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
		qcow2Header, err := qcow2.ReadHeader(httpResponse.Body)
		if err != nil {
			return nil, err
		}
		vmInfo.Volumes[0].VirtualSize = qcow2Header.Size
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
	if *imageServerHostname == "" {
		return
	}
	imageName := vmTags["RequiredImage"]
	if imageName == "" {
		return
	}
	imageServer := fmt.Sprintf("%s:%d",
		*imageServerHostname, *imageServerPortNum)
	client, err := dialImageServer(imageServer)
	if err != nil {
		logger.Println(err)
		return
	}
	defer client.Close()
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
	if *vmHostname == "" {
		if name := vmTags["Name"]; name == "" {
			return errors.New("no hostname specified")
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
		return err
	}
	request := hyper_proto.CreateVmRequest{
		DhcpTimeout:      *dhcpTimeout,
		DoNotStart:       *doNotStart,
		EnableNetboot:    *enableNetboot,
		MinimumFreeBytes: uint64(minFreeBytes),
		RoundupPower:     *roundupPower,
		SkipMemoryCheck:  *skipMemoryCheck,
		StorageIndices:   storageIndices,
		VmInfo:           *vmInfo,
	}
	if request.VmInfo.MemoryInMiB < 1 {
		request.VmInfo.MemoryInMiB = 1024
	}
	if request.VmInfo.MilliCPUs < 1 {
		request.VmInfo.MilliCPUs = 250
	}
	minimumCPUs := request.VmInfo.MilliCPUs / 1000
	if request.VmInfo.VirtualCPUs > 0 &&
		request.VmInfo.VirtualCPUs < minimumCPUs {
		return fmt.Errorf("vCPUs must be at least %d", minimumCPUs)
	}
	tmpVmInfo, err := approximateVolumesForCreateRequest(request.VmInfo)
	if err != nil {
		return err
	}
	if hypervisor, err := getHypervisorAddress(*tmpVmInfo, logger); err != nil {
		return err
	} else {
		logger.Debugf(0, "creating VM on %s\n", hypervisor)
		return createVmOnHypervisor(hypervisor, request, logger)
	}
}

func createVmInfoFromFlags() (*hyper_proto.VmInfo, error) {
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
	request hyper_proto.CreateVmRequest, logger log.DebugLogger) error {
	secondaryFstab := &bytes.Buffer{}
	var vinitParams []volumeInitParams
	if *secondaryVolumesInitParams == "" {
		vinitParams = makeVolumeInitParams(uint(len(secondaryVolumeSizes)))
	} else {
		err := json.ReadFromFile(*secondaryVolumesInitParams, &vinitParams)
		if err != nil {
			return err
		}
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
				return fmt.Errorf("VolumeInit[%d] missing Label", index)
			}
			if vinit.MountPoint == "" {
				return fmt.Errorf("VolumeInit[%d] missing MountPoint", index)
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
			return errors.New(
				"must not specify identityCertFile when specifying identityName")
		}
		if *identityKeyFile != "" {
			return errors.New(
				"must not specify identityKeyFile when specifying identityName")
		}
		request.DoNotStart = true
	} else if *identityCertFile != "" && *identityKeyFile != "" {
		identityCert, err := ioutil.ReadFile(*identityCertFile)
		if err != nil {
			return err
		}
		identityKey, err := ioutil.ReadFile(*identityKeyFile)
		if err != nil {
			return err
		}
		request.IdentityCertificate = identityCert
		request.IdentityKey = identityKey
	}
	var imageReader, userDataReader io.Reader
	if *imageName != "" {
		request.ImageName = *imageName
		request.ImageTimeout = *imageTimeout
		request.SkipBootloader = *skipBootloader
		if overlayFiles, err := loadOverlayFiles(); err != nil {
			return err
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
			return err
		} else {
			defer file.Close()
			request.ImageDataSize = uint64(size)
			imageReader = file
		}
	} else {
		return errors.New("no image specified")
	}
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
	err = callCreateVm(client, request, &reply, imageReader, userDataReader,
		int64(request.ImageDataSize), int64(request.UserDataSize), logger)
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

func loadOverlayFiles() (map[string][]byte, error) {
	if *overlayDirectory == "" {
		return nil, nil
	}
	return fsutil.ReadFileTree(*overlayDirectory, *overlayPrefix)
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

func (r *wrappedReadCloser) Close() error {
	return r.real.Close()
}

func (r *wrappedReadCloser) Read(p []byte) (n int, err error) {
	return r.wrap.Read(p)
}
