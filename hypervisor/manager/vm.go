package manager

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	imclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil/mounts"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/filelogger"
	"github.com/Cloud-Foundations/Dominator/lib/log/prefixlogger"
	"github.com/Cloud-Foundations/Dominator/lib/log/serverlogger"
	"github.com/Cloud-Foundations/Dominator/lib/log/teelogger"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	libnet "github.com/Cloud-Foundations/Dominator/lib/net"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	objclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/rsync"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	"github.com/Cloud-Foundations/Dominator/lib/verstr"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

const (
	bootlogFilename      = "bootlog"
	lastPatchLogFilename = "lastPatchLog"
	serialSockFilename   = "serial0.sock"

	rebootJson = `{ "execute": "send-key",
     "arguments": { "keys": [ { "type": "qcode", "data": "ctrl" },
                              { "type": "qcode", "data": "alt" },
                              { "type": "qcode", "data": "delete" } ] } }
`
)

var (
	carriageReturnLiteral   = []byte{'\r'}
	errorNoAccessToResource = errors.New("no access to resource")
	newlineLiteral          = []byte{'\n'}
	newlineReplacement      = []byte{'\\', 'n'}

	qemuCommand = flag.String("qemuCommand", "qemu-system-x86_64",
		"QEMU command")
)

// updateLogger is a logger that sends progress messages back to vm-control
type updateLogger struct {
	baseLogger log.DebugLogger
	conn       *srpc.Conn
	sendUpdate func(*srpc.Conn, string) error
}

func (ul *updateLogger) Debug(level uint8, v ...interface{}) {
	ul.baseLogger.Debug(level, v...)
}

func (ul *updateLogger) Debugf(level uint8, format string, v ...interface{}) {
	ul.baseLogger.Debugf(level, format, v...)
}

func (ul *updateLogger) Debugln(level uint8, v ...interface{}) {
	ul.baseLogger.Debugln(level, v...)
}

func (ul *updateLogger) Fatal(v ...interface{}) {
	ul.baseLogger.Fatal(v...)
}

func (ul *updateLogger) Fatalf(format string, v ...interface{}) {
	ul.baseLogger.Fatalf(format, v...)
}

func (ul *updateLogger) Fatalln(v ...interface{}) {
	ul.baseLogger.Fatalln(v...)
}

func (ul *updateLogger) Panic(v ...interface{}) {
	ul.baseLogger.Panic(v...)
}

func (ul *updateLogger) Panicf(format string, v ...interface{}) {
	ul.baseLogger.Panicf(format, v...)
}

func (ul *updateLogger) Panicln(v ...interface{}) {
	ul.baseLogger.Panicln(v...)
}

func (ul *updateLogger) Print(v ...interface{}) {
	ul.baseLogger.Print(v...)
	if ul.sendUpdate != nil && ul.conn != nil {
		message := fmt.Sprint(v...)
		ul.sendUpdate(ul.conn, message)
	}
}

func (ul *updateLogger) Printf(format string, v ...interface{}) {
	ul.baseLogger.Printf(format, v...)
	if ul.sendUpdate != nil && ul.conn != nil {
		message := fmt.Sprintf(format, v...)
		ul.sendUpdate(ul.conn, message)
	}
}

func (ul *updateLogger) Println(v ...interface{}) {
	ul.baseLogger.Println(v...)
	if ul.sendUpdate != nil && ul.conn != nil {
		message := fmt.Sprintln(v...)
		ul.sendUpdate(ul.conn, message)
	}
}

func checkCpuPriority(authInfo *srpc.AuthInformation, cpuPriority int) error {
	if cpuPriority < 0 && !authInfo.HaveMethodAccess {
		return fmt.Errorf("insufficient privilege to set CpuPriority=%d",
			cpuPriority)
	}
	return nil
}

func computeSize(minimumFreeBytes, roundupPower, size uint64) uint64 {
	minBytes := size + size>>3 // 12% extra for good luck.
	minBytes += minimumFreeBytes
	if roundupPower < 24 {
		roundupPower = 24 // 16 MiB.
	}
	imageUnits := minBytes >> roundupPower
	if imageUnits<<roundupPower < minBytes {
		imageUnits++
	}
	return imageUnits << roundupPower
}

// copyData will create and truncate the specified file and will copy data to
// the file. If reader is nil, the file is fallocated or zero-filled.
func copyData(filename string, reader io.Reader, length uint64,
	disableFillZero bool, logger log.DebugLogger) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY,
		fsutil.PrivateFilePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := os.Truncate(filename, int64(length)); err != nil {
		return err
	}
	if reader == nil {
		return fsutil.FallocateOrFill(filename, length, disableFillZero, logger)
	}
	_, err = io.CopyN(file, reader, int64(length))
	return err
}

func createTapDevice(bridge string) (*os.File, error) {
	bridgeIf, err := net.InterfaceByName(bridge)
	if err != nil {
		return nil, err
	}
	tapFile, tapName, err := libnet.CreateTapDevice()
	if err != nil {
		return nil, fmt.Errorf("error creating tap device: %s", err)
	}
	doAutoClose := true
	defer func() {
		if doAutoClose {
			tapFile.Close()
		}
	}()
	cmd := exec.Command("ip", "link", "set", tapName,
		"mtu", strconv.Itoa(bridgeIf.MTU),
		"up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error upping: %s: %s", err, output)
	}
	cmd = exec.Command("ip", "link", "set", tapName, "master", bridge)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error attaching: %s: %s", err, output)
	}
	doAutoClose = false
	return tapFile, nil
}

func deleteFilesNotInImage(imgFS, vmFS *filesystem.FileSystem,
	rootDir string, logger log.DebugLogger) error {
	var totalBytes uint64
	imgHashToInodesTable := imgFS.HashToInodesTable()
	imgComputedFiles := make(map[string]struct{})
	imgFS.ForEachFile(func(name string, inodeNumber uint64,
		inode filesystem.GenericInode) error {
		if _, ok := inode.(*filesystem.ComputedRegularInode); ok {
			imgComputedFiles[name] = struct{}{}
		}
		return nil
	})
	for filename, inum := range vmFS.FilenameToInodeTable() {
		if inode, ok := vmFS.InodeTable[inum].(*filesystem.RegularInode); ok {
			if inode.Size < 1 {
				continue
			}
			if _, isComputed := imgComputedFiles[filename]; isComputed {
				continue
			}
			if _, inImage := imgHashToInodesTable[inode.Hash]; inImage {
				continue
			}
			pathname := filepath.Join(rootDir, filename)
			if err := os.Remove(pathname); err != nil {
				return err
			}
			logger.Debugf(1, "pre-delete: %s\n", pathname)
			totalBytes += inode.Size
		}
	}
	logger.Debugf(0, "pre-delete: totalBytes: %s\n",
		format.FormatBytes(totalBytes))
	return nil
}

func extractKernel(volume proto.LocalVolume, extension string,
	objectsGetter objectserver.ObjectsGetter, fs *filesystem.FileSystem,
	bootInfo *util.BootInfoType) error {
	dirent := bootInfo.KernelImageDirent
	if dirent == nil {
		return errors.New("no kernel image found")
	}
	inode, ok := dirent.Inode().(*filesystem.RegularInode)
	if !ok {
		return errors.New("kernel image is not a regular file")
	}
	inode.Size = 0
	filename := filepath.Join(volume.DirectoryToCleanup, "kernel"+extension)
	_, err := objectserver.LinkObject(filename, objectsGetter, inode.Hash)
	if err != nil {
		return err
	}
	dirent = bootInfo.InitrdImageDirent
	if dirent != nil {
		inode, ok := dirent.Inode().(*filesystem.RegularInode)
		if !ok {
			return errors.New("initrd image is not a regular file")
		}
		inode.Size = 0
		filename := filepath.Join(volume.DirectoryToCleanup,
			"initrd"+extension)
		_, err = objectserver.LinkObject(filename, objectsGetter,
			inode.Hash)
		if err != nil {
			return err
		}
	}
	return nil
}

func maybeDrainAll(conn *srpc.Conn, request proto.CreateVmRequest) error {
	if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
		return err
	}
	if err := maybeDrainUserData(conn, request); err != nil {
		return err
	}
	return nil
}

func maybeDrainImage(imageReader io.Reader, imageDataSize uint64) error {
	if imageDataSize > 0 { // Drain data.
		_, err := io.CopyN(ioutil.Discard, imageReader, int64(imageDataSize))
		return err
	}
	return nil
}

func maybeDrainUserData(conn *srpc.Conn, request proto.CreateVmRequest) error {
	if request.UserDataSize > 0 { // Drain data.
		_, err := io.CopyN(ioutil.Discard, conn, int64(request.UserDataSize))
		return err
	}
	return nil
}

// numSpecifiedVirtualCPUs calculates the number of virtual CPUs required for
// the specified request. The request must be correct (i.e. sufficient vCPUs).
func numSpecifiedVirtualCPUs(milliCPUs, vCPUs uint) uint {
	nCpus := milliCPUs / 1000
	if nCpus < 1 {
		nCpus = 1
	}
	if nCpus*1000 < milliCPUs {
		nCpus++
	}
	if nCpus < vCPUs {
		nCpus = vCPUs
	}
	return nCpus
}

func readData(firstByte byte, moreBytes <-chan byte) []byte {
	buffer := make([]byte, 1, len(moreBytes)+1)
	buffer[0] = firstByte
	for {
		select {
		case char, ok := <-moreBytes:
			if !ok {
				return buffer
			}
			buffer = append(buffer, char)
		default:
			return buffer
		}
	}
}

func readOne(objectsDir string, hashVal hash.Hash, length uint64,
	reader io.Reader) error {
	filename := filepath.Join(objectsDir, objectcache.HashToFilename(hashVal))
	dirname := filepath.Dir(filename)
	if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
		return err
	}
	return fsutil.CopyToFile(filename, fsutil.PrivateFilePerms, reader, length)
}

// Returns bytes read up to a carriage return (which is discarded), and true if
// the last byte was read, else false.
func readUntilCarriageReturn(firstByte byte, moreBytes <-chan byte,
	echo chan<- byte) ([]byte, bool) {
	buffer := make([]byte, 1, len(moreBytes)+1)
	buffer[0] = firstByte
	echo <- firstByte
	for char := range moreBytes {
		echo <- char
		if char == '\r' {
			echo <- '\n'
			return buffer, false
		}
		buffer = append(buffer, char)
	}
	return buffer, true
}

// removeFile will remove the specified filename. If the removal was successful
// or the file does not exist, nil is returned, else an error is returned.
func removeFile(filename string) error {
	if err := os.Remove(filename); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func sanitiseSnapshotName(name string) (string, error) {
	if name == "" {
		return "snapshot", nil
	}
	if strings.ContainsAny(name, ".:/") {
		return "",
			fmt.Errorf("prohibited characters in snapshot name: \"%s\"", name)
	}
	return "snapshot:" + name, nil
}

func setVolumeSize(filename string, size uint64) error {
	if err := os.Truncate(filename, int64(size)); err != nil {
		return err
	}
	return fsutil.Fallocate(filename, size)
}

func (m *Manager) acknowledgeVm(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.destroyTimer.Stop()
	return nil
}

func (m *Manager) addVmVolumes(ipAddr net.IP, authInfo *srpc.AuthInformation,
	volumeSizes []uint64) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	volumes := make([]proto.Volume, 0, len(volumeSizes))
	for _, size := range volumeSizes {
		volumes = append(volumes, proto.Volume{Size: size})
	}
	volumeDirectories, err := vm.manager.getVolumeDirectories(0, 0, volumes,
		vm.SpreadVolumes, nil)
	if err != nil {
		return err
	}
	volumeLocations := make([]proto.LocalVolume, 0, len(volumes))
	defer func() {
		for _, volumeLocation := range volumeLocations {
			os.Remove(volumeLocation.Filename)
			os.Remove(volumeLocation.DirectoryToCleanup)
		}
	}()
	for index, volumeDirectory := range volumeDirectories {
		dirname := filepath.Join(volumeDirectory, vm.ipAddress)
		filename := filepath.Join(dirname, indexToName(len(vm.Volumes)+index))
		volumeLocation := proto.LocalVolume{
			DirectoryToCleanup: dirname,
			Filename:           filename,
		}
		if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
			return err
		}
		cFlags := os.O_CREATE | os.O_EXCL | os.O_RDWR
		file, err := os.OpenFile(filename, cFlags, fsutil.PrivateFilePerms)
		if err != nil {
			return err
		} else {
			file.Close()
		}
		if err := setVolumeSize(filename, volumeSizes[index]); err != nil {
			return err
		}
		volumeLocations = append(volumeLocations, volumeLocation)
	}
	vm.VolumeLocations = append(vm.VolumeLocations, volumeLocations...)
	volumeLocations = nil // Prevent cleanup. Thunderbirds are Go!
	vm.Volumes = append(vm.Volumes, volumes...)
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) allocateVm(req proto.CreateVmRequest,
	authInfo *srpc.AuthInformation) (*vmInfoType, error) {
	for _, volume := range req.Volumes {
		if err := volume.Interface.CheckValid(); err != nil {
			return nil, err
		}
	}
	dirname := filepath.Join(m.StateDir, "VMs")
	if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
		return nil, err
	}
	if err := req.ConsoleType.CheckValid(); err != nil {
		return nil, err
	}
	if err := checkCpuPriority(authInfo, req.CpuPriority); err != nil {
		return nil, err
	}
	if err := req.MachineType.CheckValid(); err != nil {
		return nil, err
	}
	if req.MemoryInMiB < 1 {
		return nil, errors.New("no memory specified")
	}
	if req.MilliCPUs < 1 {
		return nil, errors.New("no CPUs specified")
	}
	minimumCPUs := req.MilliCPUs / 1000
	if req.VirtualCPUs > 0 && req.VirtualCPUs < minimumCPUs {
		return nil, fmt.Errorf("VirtualCPUs must be at least %d", minimumCPUs)
	}
	if err := req.WatchdogAction.CheckValid(); err != nil {
		return nil, err
	}
	if err := req.WatchdogModel.CheckValid(); err != nil {
		return nil, err
	}
	subnetIDs := map[string]struct{}{req.SubnetId: {}}
	for _, subnetId := range req.SecondarySubnetIDs {
		if subnetId == "" {
			return nil,
				errors.New("cannot give unspecified secondary subnet ID")
		}
		if _, ok := subnetIDs[subnetId]; ok {
			return nil,
				fmt.Errorf("subnet: %s specified multiple times", subnetId)
		}
		subnetIDs[subnetId] = struct{}{}
	}
	address, subnetId, err := m.getFreeAddress(req.Address.IpAddress,
		req.SubnetId, authInfo)
	if err != nil {
		return nil, err
	}
	addressesToFree := []proto.Address{address}
	defer func() {
		for _, address := range addressesToFree {
			err := m.releaseAddressInPool(address)
			if err != nil {
				m.Logger.Println(err)
			}
		}
	}()
	var secondaryAddresses []proto.Address
	for index, subnetId := range req.SecondarySubnetIDs {
		var reqIpAddr net.IP
		if index < len(req.SecondaryAddresses) {
			reqIpAddr = req.SecondaryAddresses[index].IpAddress
		}
		secondaryAddress, _, err := m.getFreeAddress(reqIpAddr, subnetId,
			authInfo)
		if err != nil {
			return nil, err
		}
		secondaryAddresses = append(secondaryAddresses, secondaryAddress)
		addressesToFree = append(addressesToFree, secondaryAddress)
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.checkSufficientCPUWithLock(req.MilliCPUs); err != nil {
		return nil, err
	}
	totalMemoryInMiB := getVmInfoMemoryInMiB(req.VmInfo)
	err = m.checkSufficientMemoryWithLock(totalMemoryInMiB, nil)
	if err != nil {
		return nil, err
	}
	var ipAddress string
	if len(address.IpAddress) < 1 {
		ipAddress = "0.0.0.0"
	} else {
		ipAddress = address.IpAddress.String()
	}
	vm := &vmInfoType{
		LocalVmInfo: proto.LocalVmInfo{
			VmInfo: proto.VmInfo{
				Address:            address,
				CreatedOn:          time.Now(),
				ConsoleType:        req.ConsoleType,
				CpuPriority:        req.CpuPriority,
				DestroyOnPowerdown: req.DestroyOnPowerdown,
				DestroyProtection:  req.DestroyProtection,
				DisableVirtIO:      req.DisableVirtIO,
				ExtraKernelOptions: req.ExtraKernelOptions,
				Hostname:           req.Hostname,
				ImageName:          req.ImageName,
				ImageURL:           req.ImageURL,
				MachineType:        req.MachineType,
				MemoryInMiB:        req.MemoryInMiB,
				MilliCPUs:          req.MilliCPUs,
				OwnerGroups:        req.OwnerGroups,
				SpreadVolumes:      req.SpreadVolumes,
				SecondaryAddresses: secondaryAddresses,
				SecondarySubnetIDs: req.SecondarySubnetIDs,
				State:              proto.StateStarting,
				SubnetId:           subnetId,
				Tags:               req.Tags,
				VirtualCPUs:        req.VirtualCPUs,
				WatchdogAction:     req.WatchdogAction,
				WatchdogModel:      req.WatchdogModel,
			},
		},
		manager:          m,
		dirname:          filepath.Join(dirname, ipAddress),
		ipAddress:        ipAddress,
		logger:           prefixlogger.New(ipAddress+": ", m.Logger),
		metadataChannels: make(map[chan<- string]struct{}),
	}
	m.vms[ipAddress] = vm
	addressesToFree = nil
	return vm, nil
}

func (m *Manager) becomePrimaryVmOwner(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if vm.OwnerUsers[0] == authInfo.Username {
		return errors.New("you already are the primary owner")
	}
	ownerUsers := make([]string, 1, len(vm.OwnerUsers))
	ownerUsers[0] = authInfo.Username
	ownerUsers = append(ownerUsers, vm.OwnerUsers...)
	vm.OwnerUsers, vm.ownerUsers = stringutil.DeduplicateList(ownerUsers, false)
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmConsoleType(ipAddr net.IP,
	authInfo *srpc.AuthInformation, consoleType proto.ConsoleType) error {
	if err := consoleType.CheckValid(); err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	vm.ConsoleType = consoleType
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmCpuPriority(ipAddr net.IP,
	authInfo *srpc.AuthInformation, cpuPriority int) error {
	if err := checkCpuPriority(authInfo, cpuPriority); err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if vm.CpuPriority == cpuPriority {
		return nil
	}
	var modifyProcess bool
	switch vm.State {
	case proto.StateStarting:
		return errors.New("VM is starting")
	case proto.StateRunning, proto.StateDebugging:
		modifyProcess = true
	case proto.StateStopping:
		return errors.New("VM is stopping")
	case proto.StateStopped, proto.StateFailedToStart, proto.StateMigrating,
		proto.StateExporting, proto.StateCrashed:
	case proto.StateDestroying:
		return errors.New("VM is already destroying")
	default:
		return errors.New("unknown state: " + vm.State.String())
	}
	if modifyProcess {
		if err := vm.setCpuPriority(cpuPriority); err != nil {
			return err
		}
	}
	vm.CpuPriority = cpuPriority
	vm.writeAndSendInfo()
	return nil
}

// changeVmCPUs returns true if the number of CPUs was changed.
func (m *Manager) changeVmCPUs(vm *vmInfoType, req proto.ChangeVmSizeRequest) (
	bool, error) {
	if req.MilliCPUs < 1 {
		req.MilliCPUs = vm.MilliCPUs
	}
	if req.VirtualCPUs < 1 {
		req.VirtualCPUs = vm.VirtualCPUs
	}
	minimumCPUs := numSpecifiedVirtualCPUs(req.MilliCPUs, 0)
	if req.VirtualCPUs > 0 && req.VirtualCPUs < minimumCPUs {
		return false, fmt.Errorf("VirtualCPUs must be at least %d", minimumCPUs)
	}
	if req.MilliCPUs == vm.MilliCPUs && req.VirtualCPUs == vm.VirtualCPUs {
		return false, nil
	}
	oldCPUs := numSpecifiedVirtualCPUs(vm.MilliCPUs, vm.VirtualCPUs)
	newCPUs := numSpecifiedVirtualCPUs(req.MilliCPUs, req.VirtualCPUs)
	if oldCPUs == newCPUs {
		vm.MilliCPUs = req.MilliCPUs
		vm.VirtualCPUs = req.VirtualCPUs
		return true, nil
	}
	if vm.State != proto.StateStopped {
		return false, errors.New("VM is not stopped")
	}
	if newCPUs <= oldCPUs {
		vm.MilliCPUs = req.MilliCPUs
		vm.VirtualCPUs = req.VirtualCPUs
		return true, nil
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	err := m.checkSufficientCPUWithLock(req.MilliCPUs - vm.MilliCPUs)
	if err != nil {
		return false, err
	}
	vm.MilliCPUs = req.MilliCPUs
	vm.VirtualCPUs = req.VirtualCPUs
	return true, nil
}

func (m *Manager) changeVmDestroyProtection(ipAddr net.IP,
	authInfo *srpc.AuthInformation, destroyProtection bool) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.DestroyProtection = destroyProtection
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmMachineType(ipAddr net.IP,
	authInfo *srpc.AuthInformation, machineType proto.MachineType) error {
	if err := machineType.CheckValid(); err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	vm.MachineType = machineType
	vm.writeAndSendInfo()
	return nil
}

// changeVmMemory returns true if the memory size was changed.
func (m *Manager) changeVmMemory(vm *vmInfoType,
	memoryInMiB uint64) (bool, error) {
	if memoryInMiB == vm.MemoryInMiB {
		return false, nil
	}
	if vm.State != proto.StateStopped {
		return false, errors.New("VM is not stopped")
	}
	changed := false
	if memoryInMiB < vm.MemoryInMiB {
		vm.MemoryInMiB = memoryInMiB
		changed = true
	} else if memoryInMiB > vm.MemoryInMiB {
		m.mutex.Lock()
		err := m.checkSufficientMemoryWithLock(memoryInMiB-vm.MemoryInMiB, vm)
		if err == nil {
			vm.MemoryInMiB = memoryInMiB
			changed = true
		}
		m.mutex.Unlock()
		if err != nil {
			return changed, err
		}
	}
	return changed, nil
}

func (m *Manager) changeVmOwnerGroups(ipAddr net.IP,
	authInfo *srpc.AuthInformation, ownerGroups []string) error {
	ownerGroups, _ = stringutil.DeduplicateList(ownerGroups, false)
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.OwnerGroups = ownerGroups
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmOwnerUsers(ipAddr net.IP,
	authInfo *srpc.AuthInformation, extraUsers []string) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	ownerUsers := make([]string, 1, len(extraUsers)+1)
	ownerUsers[0] = vm.OwnerUsers[0]
	ownerUsers = append(ownerUsers, extraUsers...)
	vm.OwnerUsers, vm.ownerUsers = stringutil.DeduplicateList(ownerUsers, false)
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmSize(authInfo *srpc.AuthInformation,
	req proto.ChangeVmSizeRequest) error {
	vm, err := m.getVmLockAndAuth(req.IpAddress, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	changed := false
	if req.MemoryInMiB > 0 {
		if _changed, e := m.changeVmMemory(vm, req.MemoryInMiB); e != nil {
			err = e
		} else if _changed {
			changed = true
		}
	}
	if (req.MilliCPUs > 0 || req.VirtualCPUs > 0) && err == nil {
		if _changed, _err := m.changeVmCPUs(vm, req); _err != nil {
			err = _err
		} else if _changed {
			changed = true
		}
	}
	if changed {
		vm.writeAndSendInfo()
	}
	return err
}

func (m *Manager) changeVmSubnet(authInfo *srpc.AuthInformation,
	req proto.ChangeVmSubnetRequest) (*proto.ChangeVmSubnetResponse, error) {
	if req.SubnetId == "" {
		return nil, fmt.Errorf("no subnet specified")
	}
	vm, err := m.getStoppedVmAndRemove(req.IpAddress, authInfo, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		m.mutex.Lock()
		_, ok := m.vms[vm.ipAddress]
		m.vms[vm.ipAddress] = vm
		m.mutex.Unlock()
		if ok {
			panic(fmt.Sprintf("changeVmSubnet(%s): duplicate", vm.ipAddress))
		}
	}()
	if req.SubnetId == vm.SubnetId {
		return nil, errors.New("same subnet specified")
	}
	oldSubnetId := vm.SubnetId
	address, subnetId, err := m.getFreeAddress(nil, req.SubnetId, authInfo)
	if err != nil {
		return nil, err
	}
	ipAddress := address.IpAddress.String()
	addressToFree := address
	defer func() {
		err := m.releaseAddressInPool(addressToFree)
		if err != nil {
			vm.logger.Println(err)
		}
	}()
	response := proto.ChangeVmSubnetResponse{
		NewIpAddress:    address.IpAddress,
		OldIdentityName: vm.IdentityName,
	}
	newDirname := filepath.Join(m.StateDir, "VMs", ipAddress)
	renameMap := make(map[string]string) // Key: old, value: new.
	renameMap[vm.dirname] = newDirname
	for _, volume := range vm.VolumeLocations {
		if _, ok := renameMap[volume.DirectoryToCleanup]; ok {
			continue
		}
		renameMap[volume.DirectoryToCleanup] = filepath.Join(
			filepath.Dir(volume.DirectoryToCleanup), ipAddress)
	}
	if err := multiRename(renameMap, true); err != nil {
		return nil, err
	} // Begin critical section: it's too late to back out now. Any error will
	// result in a corrupted state.
	// Clean up old state.
	select {
	case vm.accessTokenCleanupNotifier <- struct{}{}:
	default:
	}
	for ch := range vm.metadataChannels {
		close(ch)
		delete(vm.metadataChannels, ch)
	}
	if vm.identityProviderNotifier != nil {
		close(vm.identityProviderNotifier)
		vm.identityProviderNotifier = nil
		vm.IdentityName = ""
	}
	vm.manager.DhcpServer.RemoveLease(vm.Address.IpAddress)
	vm.manager.sendVmInfo(vm.ipAddress, nil)
	if vm.lockWatcher != nil {
		vm.lockWatcher.Stop()
	}
	// Set up new state.
	oldRootLabel := vm.rootLabelSaved(false)
	addressToFree = vm.Address
	vm.Address = address
	vm.dirname = newDirname
	vm.ipAddress = ipAddress
	vm.logger = prefixlogger.New(ipAddress+": ", m.Logger)
	vm.SubnetId = subnetId
	if oldRootLabel == vm.rootLabel(false) {
		vm.RootFileSystemLabel = "" // Restoring original (default) label.
	} else {
		vm.RootFileSystemLabel = oldRootLabel
	}
	for index := range vm.VolumeLocations {
		volume := &vm.VolumeLocations[index]
		volume.DirectoryToCleanup = renameMap[volume.DirectoryToCleanup]
		volume.Filename = filepath.Join(volume.DirectoryToCleanup,
			filepath.Base(volume.Filename))
	}
	vm.writeAndSendInfo()
	vm.setupLockWatcher()
	vm.logger.Printf("changed subnet: %s -> %s, original IP: %s\n",
		oldSubnetId, subnetId, addressToFree.IpAddress)
	return &response, nil
}

func (m *Manager) changeVmTags(ipAddr net.IP, authInfo *srpc.AuthInformation,
	tgs tags.Tags) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.Tags = tgs
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmVolumeInterfaces(ipAddr net.IP,
	authInfo *srpc.AuthInformation,
	volumeInterfaces []proto.VolumeInterface) error {
	for _, volumeInterface := range volumeInterfaces {
		if err := volumeInterface.CheckValid(); err != nil {
			return err
		}
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if len(volumeInterfaces) > len(vm.Volumes) {
		return errors.New("more volume interfaces specified than VM volumes")
	}
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	for index, volumeInterface := range volumeInterfaces {
		vm.Volumes[index].Interface = volumeInterface
	}
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) changeVmVolumeSize(ipAddr net.IP,
	authInfo *srpc.AuthInformation, index uint, size uint64) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	var haveLock bool
	defer func() {
		vm.allowMutationsAndUnlock(haveLock)
	}()
	if index >= uint(len(vm.Volumes)) {
		return errors.New("invalid volume index")
	}
	volume := vm.Volumes[index]
	if volume.Format != proto.VolumeFormatRaw {
		return errors.New("cannot resize non-RAW volumes")
	}
	localVolume := vm.VolumeLocations[index]
	if size == volume.Size {
		return nil
	}
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	if size < volume.Size {
		if err := shrink2fs(localVolume.Filename, size, vm.logger); err != nil {
			return err
		}
		if err := setVolumeSize(localVolume.Filename, size); err != nil {
			return err
		}
		vm.mutex.Lock()
		haveLock = true
		vm.Volumes[index].Size = size
		vm.writeAndSendInfo()
		return nil
	}
	var statbuf syscall.Statfs_t
	if err := syscall.Statfs(localVolume.Filename, &statbuf); err != nil {
		return err
	}
	if size-volume.Size > uint64(statbuf.Bavail*uint64(statbuf.Bsize)) {
		return errors.New("not enough free space")
	}
	if err := setVolumeSize(localVolume.Filename, size); err != nil {
		return err
	}
	vm.mutex.Lock()
	vm.Volumes[index].Size = size
	vm.writeAndSendInfo()
	vm.mutex.Unlock()
	// Try and grow an ext{2,3,4} file-system. If this fails, return the error
	// to the caller, but the volume will have been expanded. Someone else can
	// deal with adjusting partitions and growing file-systems.
	return grow2fs(localVolume.Filename, vm.logger)
}

func (m *Manager) checkVmHasHealthAgent(ipAddr net.IP) (bool, error) {
	vm, err := m.getVmAndLock(ipAddr, false)
	if err != nil {
		return false, err
	}
	defer vm.mutex.RUnlock()
	if vm.State != proto.StateRunning {
		return false, nil
	}
	return vm.hasHealthAgent, nil
}

func (m *Manager) commitImportedVm(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if !vm.Uncommitted {
		return fmt.Errorf("%s is already committed", ipAddr)
	}
	if err := m.registerAddress(vm.Address); err != nil {
		return err
	}
	for _, address := range vm.SecondaryAddresses {
		if err := m.registerAddress(address); err != nil {
			return err
		}
	}
	vm.Uncommitted = false
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) connectToVmConsole(ipAddr net.IP,
	authInfo *srpc.AuthInformation) (net.Conn, error) {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return nil, err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateRunning {
		return nil, errors.New("VM is not running")
	}
	if vm.ConsoleType != proto.ConsoleVNC {
		return nil, errors.New("VNC console is not enabled")
	}
	console, err := net.Dial("unix", filepath.Join(vm.dirname, "vnc"))
	if err != nil {
		return nil, err
	}
	return console, nil
}

func (m *Manager) connectToVmManager(ipAddr net.IP) (
	chan<- byte, <-chan byte, error) {
	input := make(chan byte, 256)
	vm, err := m.getVmAndLock(ipAddr, true)
	if err != nil {
		return nil, nil, err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateRunning {
		return nil, nil, errors.New("VM is not running")
	}
	commandInput := vm.commandInput
	if commandInput == nil {
		return nil, nil, errors.New("no commandInput for VM")
	}
	// Drain any previous output.
	for keepReading := true; keepReading; {
		select {
		case <-vm.commandOutput:
		default:
			keepReading = false
			break
		}
	}
	go func(input <-chan byte, output chan<- string) {
		for char := range input {
			if char == '\r' {
				continue
			}
			buffer, gotLast := readUntilCarriageReturn(char, input,
				vm.commandOutput)
			output <- "\\" + string(buffer)
			if gotLast {
				break
			}
		}
		vm.logger.Debugln(0, "input channel for manager closed")
	}(input, vm.commandInput)
	return input, vm.commandOutput, nil
}

func (m *Manager) connectToVmSerialPort(ipAddr net.IP,
	authInfo *srpc.AuthInformation,
	portNumber uint) (chan<- byte, <-chan byte, error) {
	if portNumber > 0 {
		return nil, nil, errors.New("only one serial port is supported")
	}
	input := make(chan byte, 256)
	output := make(chan byte, 16<<10)
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return nil, nil, err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateRunning {
		return nil, nil, errors.New("VM is not running")
	}
	serialInput := vm.serialInput
	if serialInput == nil {
		return nil, nil, errors.New("no serial input device for VM")
	}
	if vm.serialOutput != nil {
		return nil, nil, errors.New("VM already has a serial port connection")
	}
	vm.serialOutput = output
	go func(input <-chan byte, output chan<- byte) {
		for char := range input {
			buffer := readData(char, input)
			if _, err := serialInput.Write(buffer); err != nil {
				vm.logger.Printf("error writing to serial port: %s\n", err)
				break
			}
		}
		vm.logger.Debugln(0, "input channel for console closed")
		vm.mutex.Lock()
		if vm.serialOutput != nil {
			close(vm.serialOutput)
			vm.serialOutput = nil
		}
		vm.mutex.Unlock()
	}(input, output)
	return input, output, nil
}

func (m *Manager) copyVm(conn *srpc.Conn, request proto.CopyVmRequest) error {
	m.Logger.Debugf(1, "CopyVm(%s) starting\n", conn.Username())
	// Need to shrink IP address for later generation of root file-system label.
	request.IpAddress = proto.ShrinkIP(request.IpAddress)
	hypervisor, err := srpc.DialHTTP("tcp", request.SourceHypervisor, 0)
	if err != nil {
		return err
	}
	defer hypervisor.Close()
	defer func() {
		req := proto.DiscardVmAccessTokenRequest{
			AccessToken: request.AccessToken,
			IpAddress:   request.IpAddress}
		var reply proto.DiscardVmAccessTokenResponse
		hypervisor.RequestReply("Hypervisor.DiscardVmAccessToken",
			req, &reply)
	}()
	getInfoRequest := proto.GetVmInfoRequest{request.IpAddress}
	var getInfoReply proto.GetVmInfoResponse
	err = hypervisor.RequestReply("Hypervisor.GetVmInfo", getInfoRequest,
		&getInfoReply)
	if err != nil {
		return err
	}
	switch getInfoReply.VmInfo.State {
	case proto.StateStopped, proto.StateRunning:
	default:
		return errors.New("VM is not stopped or running")
	}
	accessToken := request.AccessToken
	ownerUsers := make([]string, 1, len(request.OwnerUsers)+1)
	ownerUsers[0] = conn.Username()
	if ownerUsers[0] == "" {
		return errors.New("no authentication data")
	}
	ownerUsers = append(ownerUsers, request.OwnerUsers...)
	vmInfo := request.VmInfo
	vmInfo.Address = proto.Address{}
	vmInfo.SecondaryAddresses = nil
	vmInfo.Uncommitted = false
	vmInfo.Volumes = getInfoReply.VmInfo.Volumes
	vm, err := m.allocateVm(proto.CreateVmRequest{VmInfo: vmInfo},
		conn.GetAuthInformation())
	if err != nil {
		return err
	}
	defer func() { // Evaluate vm at return time, not defer time.
		vm.cleanup()
	}()
	vm.OwnerUsers, vm.ownerUsers = stringutil.DeduplicateList(ownerUsers, false)
	vm.Volumes = vmInfo.Volumes
	if !request.SkipMemoryCheck {
		if err := <-tryAllocateMemory(vmInfo.MemoryInMiB); err != nil {
			return err
		}
	}
	var secondaryVolumes []proto.Volume
	for index, volume := range vmInfo.Volumes {
		if index > 0 {
			secondaryVolumes = append(secondaryVolumes, volume)
		}
	}
	err = vm.setupVolumes(vmInfo.Volumes[0].Size, vmInfo.Volumes[0].Type,
		secondaryVolumes, vmInfo.SpreadVolumes, nil)
	if err != nil {
		return err
	}
	if err := os.Mkdir(vm.dirname, fsutil.DirPerms); err != nil {
		return err
	}
	// Begin copying over the volumes.
	err = sendVmCopyMessage(conn, "initial volume(s) copy")
	if err != nil {
		return err
	}
	err = vm.migrateVmVolumes(hypervisor, request.IpAddress, accessToken, true)
	if err != nil {
		return err
	}
	if getInfoReply.VmInfo.State != proto.StateStopped {
		err = sendVmCopyMessage(conn, "stopping VM")
		if err != nil {
			return err
		}
		err := hyperclient.StopVm(hypervisor, request.IpAddress,
			request.AccessToken)
		if err != nil {
			return err
		}
		defer hyperclient.StartVm(hypervisor, request.IpAddress, accessToken)
		err = sendVmCopyMessage(conn, "update volume(s)")
		if err != nil {
			return err
		}
		err = vm.migrateVmVolumes(hypervisor, request.IpAddress, accessToken,
			false)
		if err != nil {
			return err
		}
	}
	// Copy old root file-system label.
	oldVm := &vmInfoType{}
	oldVm.Address.IpAddress = request.IpAddress
	oldVm.RootFileSystemLabel = getInfoReply.VmInfo.RootFileSystemLabel
	oldRootLabel := oldVm.rootLabelSaved(false)
	if oldRootLabel == vm.rootLabel(false) {
		vm.RootFileSystemLabel = "" // Restoring original (default) label.
	} else {
		vm.RootFileSystemLabel = oldRootLabel
	}
	err = migratevmUserData(hypervisor,
		filepath.Join(vm.dirname, UserDataFile),
		request.IpAddress, accessToken)
	if err != nil {
		return err
	}
	vm.setState(proto.StateStopped)
	vm.destroyTimer = time.AfterFunc(time.Second*15, vm.autoDestroy)
	response := proto.CopyVmResponse{
		Final:     true,
		IpAddress: vm.Address.IpAddress,
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	vm.setupLockWatcher()
	vm = nil // Cancel cleanup.
	m.Logger.Debugln(1, "CopyVm() finished")
	return nil
}

func (m *Manager) createVm(conn *srpc.Conn) error {

	sendError := func(conn *srpc.Conn, err error) error {
		m.Logger.Debugf(1, "CreateVm(%s) failed: %s\n", conn.Username(), err)
		return conn.Encode(proto.CreateVmResponse{Error: err.Error()})
	}

	var ipAddressToSend net.IP
	sendUpdate := func(conn *srpc.Conn, message string) error {
		response := proto.CreateVmResponse{
			IpAddress:       ipAddressToSend,
			ProgressMessage: message,
		}
		if err := conn.Encode(response); err != nil {
			return err
		}
		return conn.Flush()
	}

	m.Logger.Debugf(1, "CreateVm(%s) starting\n", conn.Username())
	var request proto.CreateVmRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}

	// Create an update logger that sends progress messages back to vm-control
	updateLog := &updateLogger{
		baseLogger: m.Logger,
		conn:       conn,
		sendUpdate: sendUpdate,
	}
	if m.disabled {
		if err := maybeDrainAll(conn, request); err != nil {
			return err
		}
		return sendError(conn, errors.New("Hypervisor is disabled"))
	}
	ownerUsers := make([]string, 1, len(request.OwnerUsers)+1)
	ownerUsers[0] = conn.Username()
	if ownerUsers[0] == "" {
		if err := maybeDrainAll(conn, request); err != nil {
			return err
		}
		return sendError(conn, errors.New("no authentication data"))
	}
	ownerUsers = append(ownerUsers, request.OwnerUsers...)
	var identityExpires time.Time
	var identityName string
	if len(request.IdentityCertificate) > 0 && len(request.IdentityKey) > 0 {
		var err error
		var tlsCert *tls.Certificate
		tlsCert, identityName, err = validateIdentityKeyPair(
			request.IdentityCertificate, request.IdentityKey, ownerUsers[0])
		if err != nil {
			if err := maybeDrainAll(conn, request); err != nil {
				return err
			}
			return sendError(conn, err)
		}
		identityExpires = tlsCert.Leaf.NotAfter
	}
	vm, err := m.allocateVm(request, conn.GetAuthInformation())
	if err != nil {
		if err := maybeDrainAll(conn, request); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	defer func() {
		vm.cleanup() // Evaluate vm at return time, not defer time.
	}()
	vm.IdentityExpires = identityExpires
	vm.IdentityName = identityName
	var memoryError <-chan error
	if !request.SkipMemoryCheck {
		memoryError = tryAllocateMemory(getVmInfoMemoryInMiB(request.VmInfo))
	}
	vm.OwnerUsers, vm.ownerUsers = stringutil.DeduplicateList(ownerUsers, false)
	if err := os.Mkdir(vm.dirname, fsutil.DirPerms); err != nil {
		if err := maybeDrainAll(conn, request); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	err = writeKeyPair(request.IdentityCertificate, request.IdentityKey,
		filepath.Join(vm.dirname, IdentityRsaX509CertFile),
		filepath.Join(vm.dirname, IdentityRsaX509KeyFile))
	if err != nil {
		if err := maybeDrainAll(conn, request); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	var rootVolumeType proto.VolumeType
	if len(request.Volumes) > 0 {
		rootVolumeType = request.Volumes[0].Type
	}
	if request.ImageName != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		if err := sendUpdate(conn, "getting image"); err != nil {
			return err
		}
		client, img, imageName, err := m.getImage(request.ImageName,
			request.ImageTimeout)
		if err != nil {
			return sendError(conn, err)
		}
		defer client.Close()
		fs := img.FileSystem
		vm.ImageName = imageName
		size := computeSize(request.MinimumFreeBytes, request.RoundupPower,
			fs.EstimateUsage(0))
		err = vm.setupVolumes(size, rootVolumeType, request.SecondaryVolumes,
			request.SpreadVolumes, request.StorageIndices)
		if err != nil {
			return sendError(conn, err)
		}
		if err := sendUpdate(conn, "unpacking image: "+imageName); err != nil {
			return err
		}
		writeRawOptions := util.WriteRawOptions{
			DisableFillZero:    m.DisableFillZero,
			ExtraKernelOptions: request.ExtraKernelOptions,
			InitialImageName:   imageName,
			MinimumFreeBytes:   request.MinimumFreeBytes,
			OverlayDirectories: request.OverlayDirectories,
			OverlayFiles:       request.OverlayFiles,
			RootLabel:          vm.rootLabel(false),
			RoundupPower:       request.RoundupPower,
		}
		err = m.writeRaw(vm.VolumeLocations[0], "", client, fs, writeRawOptions,
			request.SkipBootloader)
		if err != nil {
			return sendError(conn, err)
		}
		if fi, err := os.Stat(vm.VolumeLocations[0].Filename); err != nil {
			return sendError(conn, err)
		} else {
			vm.Volumes = []proto.Volume{{Size: uint64(fi.Size())}}
		}
	} else if request.ImageDataSize > 0 {
		err := vm.copyRootVolume(request, conn, request.ImageDataSize,
			rootVolumeType)
		if err != nil {
			return err
		}
	} else if request.ImageURL != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		httpResponse, err := http.Get(request.ImageURL)
		if err != nil {
			return sendError(conn, err)
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			return sendError(conn, errors.New(httpResponse.Status))
		}
		if httpResponse.ContentLength < 0 {
			return sendError(conn,
				errors.New("ContentLength from: "+request.ImageURL))
		}
		err = vm.copyRootVolume(request, httpResponse.Body,
			uint64(httpResponse.ContentLength), rootVolumeType)
		if err != nil {
			return sendError(conn, err)
		}
	} else if request.MinimumFreeBytes > 0 { // Create empty root volume.
		err = vm.copyRootVolume(request, nil, request.MinimumFreeBytes,
			rootVolumeType)
		if err != nil {
			return sendError(conn, err)
		}
	} else {
		return sendError(conn, errors.New("no image specified"))
	}
	if len(request.Volumes) > 0 {
		vm.Volumes[0].Interface = request.Volumes[0].Interface
	}
	vm.Volumes[0].Type = rootVolumeType
	if request.UserDataSize > 0 {
		filename := filepath.Join(vm.dirname, UserDataFile)
		// Create a teelogger so that we get progress messages back to vm-control
		tlogger := teelogger.New(vm.logger, updateLog)
		err := copyData(filename, conn, request.UserDataSize, m.DisableFillZero, tlogger)
		if err != nil {
			return sendError(conn, err)
		}
	}
	if len(request.SecondaryVolumes) > 0 {
		err := sendUpdate(conn, "creating secondary volumes")
		if err != nil {
			return err
		}
		for index, volume := range request.SecondaryVolumes {
			fname := vm.VolumeLocations[index+1].Filename
			var dataReader io.Reader
			if request.SecondaryVolumesData {
				dataReader = conn
			}
			// Create a teelogger so that we get progress messages back to vm-control
			tlogger := teelogger.New(vm.logger, updateLog)
			err := copyData(fname, dataReader, volume.Size, m.DisableFillZero, tlogger)
			if err != nil {
				return sendError(conn, err)
			}
			if dataReader == nil && index < len(request.SecondaryVolumesInit) {
				vinit := request.SecondaryVolumesInit[index]
				err := util.MakeExt4fsWithParams(fname, util.MakeExt4fsParams{
					NoDiscard:                true,
					BytesPerInode:            vinit.BytesPerInode,
					Label:                    vinit.Label,
					ReservedBlocksPercentage: vinit.ReservedBlocksPercentage,
					Size:                     volume.Size,
				},
					vm.logger)
				if err != nil {
					return sendError(conn, err)
				}
			}
			vm.Volumes = append(vm.Volumes, volume)
		}
	}
	if memoryError != nil {
		if len(memoryError) < 1 {
			msg := "waiting for test memory allocation"
			sendUpdate(conn, msg)
			vm.logger.Debugln(0, msg)
		}
		if err := <-memoryError; err != nil {
			return sendError(conn, err)
		}
	}
	var dhcpTimedOut bool
	if request.DoNotStart {
		vm.setState(proto.StateStopped)
	} else {
		if vm.ipAddress == "" {
			ipAddressToSend = net.ParseIP(vm.ipAddress)
			if err := sendUpdate(conn, "starting VM"); err != nil {
				return err
			}
		} else {
			ipAddressToSend = net.ParseIP(vm.ipAddress)
			if err := sendUpdate(conn, "starting VM "+vm.ipAddress); err != nil {
				return err
			}
		}
		dhcpTimedOut, err = vm.startManaging(request.DhcpTimeout,
			request.EnableNetboot, false)
		if err != nil {
			return sendError(conn, err)
		}
	}
	vm.destroyTimer = time.AfterFunc(time.Second*15, vm.autoDestroy)
	response := proto.CreateVmResponse{
		DhcpTimedOut: dhcpTimedOut,
		Final:        true,
		IpAddress:    net.ParseIP(vm.ipAddress),
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	vm.setupLockWatcher()
	m.Logger.Debugf(1, "CreateVm(%s) finished, IP=%s\n",
		conn.Username(), vm.ipAddress)
	vm = nil // Cancel cleanup.
	return nil
}

func (m *Manager) debugVmImage(conn *srpc.Conn,
	authInfo *srpc.AuthInformation) error {

	sendError := func(conn *srpc.Conn, err error) error {
		return conn.Encode(proto.DebugVmImageResponse{Error: err.Error()})
	}

	sendUpdate := func(conn *srpc.Conn, message string) error {
		response := proto.DebugVmImageResponse{
			ProgressMessage: message,
		}
		if err := conn.Encode(response); err != nil {
			return err
		}
		return conn.Flush()
	}

	var request proto.DebugVmImageRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	m.Logger.Debugf(1, "DebugVmImage(%s) starting\n", request.IpAddress)
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	vm.blockMutations = true
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if len(vm.Address.IpAddress) < 1 {
			err = errors.New("cannot stop VM with externally managed lease")
		}
	default:
		err = errors.New("VM is not running or stopped")
	}
	if err != nil {
		vm.allowMutationsAndUnlock(true)
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	rootFilename := vm.VolumeLocations[0].Filename + ".debug"
	vm.mutex.Unlock()
	haveLock := false
	doCleanup := true
	defer func() {
		if doCleanup {
			os.Remove(rootFilename)
		}
		vm.allowMutationsAndUnlock(haveLock)
	}()
	if request.ImageName != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return sendError(conn, err)
		}
		if err := sendUpdate(conn, "getting image"); err != nil {
			return sendError(conn, err)
		}
		client, img, imageName, err := m.getImage(request.ImageName,
			request.ImageTimeout)
		if err != nil {
			return sendError(conn, err)
		}
		defer client.Close()
		fs := img.FileSystem
		if err := sendUpdate(conn, "unpacking image: "+imageName); err != nil {
			return err
		}
		writeRawOptions := util.WriteRawOptions{
			InitialImageName: imageName,
			MinimumFreeBytes: request.MinimumFreeBytes,
			OverlayFiles:     request.OverlayFiles,
			RootLabel:        vm.rootLabel(true),
			RoundupPower:     request.RoundupPower,
		}
		err = m.writeRaw(vm.VolumeLocations[0], ".debug", client, fs,
			writeRawOptions, false)
		if err != nil {
			return sendError(conn, err)
		}
	} else if request.ImageDataSize > 0 {
		err := copyData(rootFilename, conn, request.ImageDataSize, m.DisableFillZero, vm.logger)
		if err != nil {
			return sendError(conn, err)
		}
	} else if request.ImageURL != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return sendError(conn, err)
		}
		httpResponse, err := http.Get(request.ImageURL)
		if err != nil {
			return sendError(conn, err)
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			return sendError(conn, errors.New(httpResponse.Status))
		}
		if httpResponse.ContentLength < 0 {
			return sendError(conn,
				errors.New("ContentLength from: "+request.ImageURL))
		}
		err = copyData(rootFilename, httpResponse.Body,
			uint64(httpResponse.ContentLength), m.DisableFillZero, vm.logger)
		if err != nil {
			return sendError(conn, err)
		}
	} else {
		return sendError(conn, errors.New("no image specified"))
	}
	vm.mutex.Lock()
	haveLock = true
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if err := sendUpdate(conn, "stopping VM"); err != nil {
			return err
		}
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.setState(proto.StateStopping)
		vm.commandInput <- "system_powerdown"
		time.AfterFunc(time.Second*15, vm.kill)
		vm.mutex.Unlock()
		<-stoppedNotifier
		vm.mutex.Lock()
		if vm.State != proto.StateStopped {
			return sendError(conn,
				errors.New("VM is not stopped after stop attempt"))
		}
	default:
		return errors.New("VM is not running or stopped")
	}
	vm.writeAndSendInfo()
	vm.setState(proto.StateStarting)
	vm.mutex.Unlock()
	haveLock = false
	sendUpdate(conn, "starting VM")
	_, err = vm.startManaging(0, false, false)
	if err != nil {
		sendError(conn, err)
	}
	response := proto.DebugVmImageResponse{
		Final: true,
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	doCleanup = false
	return nil
}

func (m *Manager) deleteVmVolume(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte, volumeIndex uint) error {
	if volumeIndex < 1 {
		return errors.New("cannot delete root volume")
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if volumeIndex >= uint(len(vm.VolumeLocations)) {
		return errors.New("volume index too large")
	}
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	if err := os.Remove(vm.VolumeLocations[volumeIndex].Filename); err != nil {
		return err
	}
	os.Remove(vm.VolumeLocations[volumeIndex].DirectoryToCleanup)
	volumeLocations := make([]proto.LocalVolume, 0, len(vm.VolumeLocations)-1)
	volumes := make([]proto.Volume, 0, len(vm.VolumeLocations)-1)
	for index, volume := range vm.VolumeLocations {
		if uint(index) != volumeIndex {
			volumeLocations = append(volumeLocations, volume)
			volumes = append(volumes, vm.Volumes[index])
		}
	}
	vm.VolumeLocations = volumeLocations
	vm.Volumes = volumes
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) destroyVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	switch vm.State {
	case proto.StateStarting:
		return errors.New("VM is starting")
	case proto.StateRunning, proto.StateDebugging:
		if vm.DestroyProtection {
			return errors.New("cannot destroy running VM when protected")
		}
		vm.setState(proto.StateDestroying)
		vm.commandInput <- "quit"
	case proto.StateStopping:
		return errors.New("VM is stopping")
	case proto.StateStopped, proto.StateFailedToStart, proto.StateMigrating,
		proto.StateExporting, proto.StateCrashed:
		vm.delete()
	case proto.StateDestroying:
		return errors.New("VM is already destroying")
	default:
		return errors.New("unknown state: " + vm.State.String())
	}
	return nil
}

func (m *Manager) discardVmAccessToken(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	for index := range vm.accessToken { // Scrub token.
		vm.accessToken[index] = 0
	}
	vm.accessToken = nil
	return nil
}

func (m *Manager) discardVmOldImage(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	extension := ".old"
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	if err := removeFile(vm.getInitrdPath() + extension); err != nil {
		return err
	}
	if err := removeFile(vm.getKernelPath() + extension); err != nil {
		return err
	}
	return removeFile(vm.VolumeLocations[0].Filename + extension)
}

func (m *Manager) discardVmOldUserData(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	return removeFile(filepath.Join(vm.dirname, UserDataFile+".old"))
}

func (m *Manager) discardVmSnapshot(ipAddr net.IP,
	authInfo *srpc.AuthInformation, snapshotName string) error {
	snapshotSuffix, err := sanitiseSnapshotName(snapshotName)
	if err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	changed, err := vm.discardSnapshot(snapshotName, snapshotSuffix)
	if changed {
		vm.writeAndSendInfo()
	}
	return err
}

func (m *Manager) exportLocalVm(authInfo *srpc.AuthInformation,
	request proto.ExportLocalVmRequest) (*proto.ExportLocalVmInfo, error) {
	if !bytes.Equal(m.rootCookie, request.VerificationCookie) {
		return nil, fmt.Errorf("bad verification cookie: you are not root")
	}
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		return nil, err
	}
	defer vm.mutex.Unlock()
	if vm.State != proto.StateStopped {
		return nil, errors.New("VM is not stopped")
	}
	bridges, _, err := vm.getBridgesAndOptions(false)
	if err != nil {
		return nil, err
	}
	vm.setState(proto.StateExporting)
	vmInfo := proto.ExportLocalVmInfo{
		Bridges:     bridges,
		LocalVmInfo: vm.LocalVmInfo,
	}
	return &vmInfo, nil
}

func (m *Manager) getImage(searchName string, imageTimeout time.Duration) (
	*srpc.Client, *image.Image, string, error) {
	client, err := srpc.DialHTTP("tcp", m.ImageServerAddress, 0)
	if err != nil {
		return nil, nil, "",
			fmt.Errorf("error connecting to image server: %s: %s",
				m.ImageServerAddress, err)
	}
	doClose := true
	defer func() {
		if doClose {
			client.Close()
		}
	}()
	if isDir, err := imclient.CheckDirectory(client, searchName); err != nil {
		return nil, nil, "", err
	} else if isDir {
		imageName, err := imclient.FindLatestImage(client, searchName, false)
		if err != nil {
			return nil, nil, "", err
		}
		if imageName == "" {
			return nil, nil, "",
				errors.New("no images in directory: " + searchName)
		}
		img, err := imclient.GetImage(client, imageName)
		if err != nil {
			return nil, nil, "", err
		}
		img.FileSystem.RebuildInodePointers()
		doClose = false
		return client, img, imageName, nil
	}
	img, err := imclient.GetImageWithTimeout(client, searchName, imageTimeout)
	if err != nil {
		return nil, nil, "", err
	}
	if img == nil {
		return nil, nil, "", errors.New("timeout getting image")
	}
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return nil, nil, "", err
	}
	doClose = false
	return client, img, searchName, nil
}

func (m *Manager) getNumVMs() (uint, uint) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.getNumVMsWithLock()
}

func (m *Manager) getNumVMsWithLock() (uint, uint) {
	var numRunning, numStopped uint
	for _, vm := range m.vms {
		if vm.State == proto.StateRunning {
			numRunning++
		} else {
			numStopped++
		}
	}
	return numRunning, numStopped
}

// getStoppedtVmAndRemove will get the specified VM and remove it from the
// Manager. The VM must be stopped.
// The Manager and VM locks are grabbed and released.
func (m *Manager) getStoppedVmAndRemove(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte) (*vmInfoType, error) {
	ipStr := ipAddr.String()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if vm := m.vms[ipStr]; vm == nil {
		return nil, fmt.Errorf("no VM with IP address: %s found", ipStr)
	} else {
		vm.mutex.Lock()
		defer vm.mutex.Unlock()
		if vm.State != proto.StateStopped {
			return nil, errors.New("VM is not stopped")
		}
		if err := vm.checkAuth(authInfo, accessToken); err != nil {
			return nil, err
		}
		if vm.blockMutations {
			return nil, errors.New("mutations blocked")
		}
		delete(m.vms, ipStr)
		return vm, nil
	}
}

func (m *Manager) getVmAccessToken(ipAddr net.IP,
	authInfo *srpc.AuthInformation, lifetime time.Duration) ([]byte, error) {
	if lifetime < time.Minute {
		return nil, errors.New("lifetime is less than 1 minute")
	}
	if lifetime > time.Hour*24 {
		return nil, errors.New("lifetime is greater than 1 day")
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return nil, err
	}
	defer vm.mutex.Unlock()
	if vm.accessToken != nil {
		return nil, errors.New("someone else has the access token")
	}
	vm.accessToken = nil
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	vm.accessToken = token
	cleanupNotifier := make(chan struct{}, 1)
	vm.accessTokenCleanupNotifier = cleanupNotifier
	go func() {
		timer := time.NewTimer(lifetime)
		select {
		case <-timer.C:
		case <-cleanupNotifier:
		}
		vm.mutex.Lock()
		defer vm.mutex.Unlock()
		for index := 0; index < len(vm.accessToken); index++ {
			vm.accessToken[index] = 0 // Scrub sensitive data.
		}
		vm.accessToken = nil
	}()
	return token, nil
}

func (m *Manager) getVmAndLock(ipAddr net.IP, write bool) (*vmInfoType, error) {
	ipStr := ipAddr.String()
	m.mutex.RLock()
	if vm := m.vms[ipStr]; vm == nil {
		m.mutex.RUnlock()
		return nil, fmt.Errorf("no VM with IP address: %s found", ipStr)
	} else {
		if write {
			vm.mutex.Lock()
		} else {
			vm.mutex.RLock()
		}
		m.mutex.RUnlock()
		return vm, nil
	}
}

func (m *Manager) getVmLockAndAuth(ipAddr net.IP, write bool,
	authInfo *srpc.AuthInformation, accessToken []byte) (*vmInfoType, error) {
	vm, err := m.getVmAndLock(ipAddr, write)
	if err != nil {
		return nil, err
	}
	if err := vm.checkAuth(authInfo, accessToken); err != nil {
		if write {
			vm.mutex.Unlock()
		} else {
			vm.mutex.RUnlock()
		}
		return nil, err
	}
	if write {
		if vm.blockMutations {
			vm.mutex.Unlock()
			return nil, errors.New("mutations blocked")
		}
	}
	return vm, nil
}

func (m *Manager) getVmBootLog(ipAddr net.IP) (io.ReadCloser, error) {
	vm, err := m.getVmAndLock(ipAddr, false)
	if err != nil {
		return nil, err
	}
	filename := filepath.Join(vm.dirname, "bootlog")
	vm.mutex.RUnlock()
	return os.Open(filename)
}

func (m *Manager) getVmFileReader(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte, filename string) (io.ReadCloser, uint64, error) {
	filename = filepath.Clean(filename)
	vm, err := m.getVmLockAndAuth(ipAddr, false, authInfo, accessToken)
	if err != nil {
		return nil, 0, err
	}
	pathname := filepath.Join(vm.dirname, filename)
	vm.mutex.RUnlock()
	if file, err := os.Open(pathname); err != nil {
		return nil, 0, err
	} else if fi, err := file.Stat(); err != nil {
		return nil, 0, err
	} else {
		return file, uint64(fi.Size()), nil
	}
}

func (m *Manager) getVmInfo(ipAddr net.IP) (proto.VmInfo, error) {
	vm, err := m.getVmAndLock(ipAddr, false)
	if err != nil {
		return proto.VmInfo{}, err
	}
	defer vm.mutex.RUnlock()
	return vm.VmInfo, nil
}

func (m *Manager) getVmInfos(request proto.GetVmInfosRequest) (
	[]proto.VmInfo, error) {
	var vmInfos []proto.VmInfo
	m.iterateOverVMs(request.IgnoreStateMask, request.VmTagsToMatch,
		request.OwnerGroups, request.OwnerUsers,
		func(ipAddr string, vm *vmInfoType) {
			vmInfos = append(vmInfos, vm.LocalVmInfo.VmInfo)
		})
	return vmInfos, nil
}

func (m *Manager) getVmLastPatchLog(ipAddr net.IP) (
	io.ReadCloser, uint64, time.Time, error) {
	vm, err := m.getVmAndLock(ipAddr, false)
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	defer vm.mutex.RUnlock()
	file, err := openBufferedFile(filepath.Join(
		vm.VolumeLocations[0].DirectoryToCleanup, lastPatchLogFilename))
	if err != nil {
		return nil, 0, time.Time{}, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, time.Time{}, err
	}
	return file, uint64(fi.Size()), fi.ModTime(), nil
}

func (m *Manager) getVmLockWatcher(ipAddr net.IP) (
	*lockwatcher.LockWatcher, error) {
	vm, err := m.getVmAndLock(ipAddr, false)
	if err != nil {
		return nil, err
	}
	defer vm.mutex.RUnlock()
	return vm.lockWatcher, nil
}

func (m *Manager) getVmVolume(conn *srpc.Conn) error {
	var request proto.GetVmVolumeRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(request.IpAddress, true,
		conn.GetAuthInformation(), request.AccessToken)
	if err != nil {
		return conn.Encode(proto.GetVmVolumeResponse{Error: err.Error()})
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	var initrd, kernel []byte
	if request.VolumeIndex == 0 {
		if initrdPath := vm.getActiveInitrdPath(); initrdPath != "" {
			if !request.GetExtraFiles && !request.IgnoreExtraFiles {
				return conn.Encode(proto.GetVmVolumeResponse{
					Error: "cannot get root volume with separate initrd"})
			}
			if request.GetExtraFiles {
				initrd, err = ioutil.ReadFile(initrdPath)
				if err != nil {
					return conn.Encode(
						proto.GetVmVolumeResponse{Error: err.Error()})
				}
			}
		}
		if kernelPath := vm.getActiveKernelPath(); kernelPath != "" {
			if !request.GetExtraFiles && !request.IgnoreExtraFiles {
				return conn.Encode(proto.GetVmVolumeResponse{
					Error: "cannot get root volume with separate kernel"})
			}
			if request.GetExtraFiles {
				kernel, err = ioutil.ReadFile(kernelPath)
				if err != nil {
					return conn.Encode(
						proto.GetVmVolumeResponse{Error: err.Error()})
				}
			}
		}
	}
	response := proto.GetVmVolumeResponse{}
	if len(initrd) > 0 || len(kernel) > 0 {
		response.ExtraFiles = make(map[string][]byte)
		response.ExtraFiles["initrd"] = initrd
		response.ExtraFiles["kernel"] = kernel
	}
	if request.VolumeIndex >= uint(len(vm.VolumeLocations)) {
		return conn.Encode(proto.GetVmVolumeResponse{
			Error: "index too large"})
	}
	file, err := os.Open(vm.VolumeLocations[request.VolumeIndex].Filename)
	if err != nil {
		return conn.Encode(proto.GetVmVolumeResponse{Error: err.Error()})
	}
	defer file.Close()
	if err := conn.Encode(response); err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	return rsync.ServeBlocks(conn, conn, conn, file,
		vm.Volumes[request.VolumeIndex].Size)
}

func (m *Manager) holdVmLock(ipAddr net.IP, timeout time.Duration,
	writeLock bool, authInfo *srpc.AuthInformation) error {
	if timeout > time.Minute {
		return fmt.Errorf("timeout: %s exceeds one minute", timeout)
	}
	if authInfo == nil {
		return fmt.Errorf("no authentication information")
	}
	vm, err := m.getVmAndLock(ipAddr, writeLock)
	if err != nil {
		return err
	}
	if writeLock {
		vm.logger.Printf("HoldVmLock(%s) by %s for writing\n",
			format.Duration(timeout), authInfo.Username)
		time.Sleep(timeout)
		vm.mutex.Unlock()
	} else {
		vm.logger.Printf("HoldVmLock(%s) by %s for reading\n",
			format.Duration(timeout), authInfo.Username)
		time.Sleep(timeout)
		vm.mutex.RUnlock()
	}
	return nil
}

func (m *Manager) importLocalVm(authInfo *srpc.AuthInformation,
	request proto.ImportLocalVmRequest) error {
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return err
	}
	requestedIpAddrs := make(map[string]struct{},
		1+len(request.SecondaryAddresses))
	requestedMacAddrs := make(map[string]struct{},
		1+len(request.SecondaryAddresses))
	requestedIpAddrs[request.Address.IpAddress.String()] = struct{}{}
	requestedMacAddrs[request.Address.MacAddress] = struct{}{}
	for _, addr := range request.SecondaryAddresses {
		ipAddr := addr.IpAddress.String()
		if _, ok := requestedIpAddrs[ipAddr]; ok {
			return fmt.Errorf("duplicate address: %s", ipAddr)
		}
		requestedIpAddrs[ipAddr] = struct{}{}
		if _, ok := requestedMacAddrs[addr.MacAddress]; ok {
			return fmt.Errorf("duplicate address: %s", addr.MacAddress)
		}
		requestedIpAddrs[addr.MacAddress] = struct{}{}
	}
	if !bytes.Equal(m.rootCookie, request.VerificationCookie) {
		return fmt.Errorf("bad verification cookie: you are not root")
	}
	request.VmInfo.OwnerUsers = []string{authInfo.Username}
	request.VmInfo.Uncommitted = true
	volumeDirectories := make([]string, 0, len(request.VolumeFilenames))
	volumes := make([]proto.Volume, 0, len(request.VolumeFilenames))
	for index, filename := range request.VolumeFilenames {
		mountEntry := mountTable.FindEntry(filename)
		if mountEntry == nil {
			return fmt.Errorf("unable to find mount entry for: %s", filename)
		}
		dirname := filepath.Join(mountEntry.MountPoint, "hyper-volumes")
		if fi, err := os.Lstat(dirname); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
				return err
			}
			m.Logger.Printf("Created directory: %s\n", dirname)
		} else if !fi.IsDir() {
			return fmt.Errorf("%s is not a directory", dirname)
		}
		volumeDirectories = append(volumeDirectories, dirname)
		if fi, err := os.Lstat(filename); err != nil {
			return err
		} else if fi.Mode()&os.ModeType != 0 {
			return fmt.Errorf("%s is not a regular file", filename)
		} else {
			var volumeFormat proto.VolumeFormat
			if index < len(request.VmInfo.Volumes) {
				volumeFormat = request.VmInfo.Volumes[index].Format
			}
			volumes = append(volumes, proto.Volume{
				Size:   uint64(fi.Size()),
				Format: volumeFormat,
			})
		}
	}
	request.Volumes = volumes
	if !request.SkipMemoryCheck {
		err := <-tryAllocateMemory(getVmInfoMemoryInMiB(request.VmInfo))
		if err != nil {
			return err
		}
	}
	ipAddress := request.Address.IpAddress.String()
	vm := &vmInfoType{
		LocalVmInfo: proto.LocalVmInfo{
			VmInfo: request.VmInfo,
		},
		manager:          m,
		dirname:          filepath.Join(m.StateDir, "VMs", ipAddress),
		ipAddress:        ipAddress,
		ownerUsers:       map[string]struct{}{authInfo.Username: {}},
		logger:           prefixlogger.New(ipAddress+": ", m.Logger),
		metadataChannels: make(map[chan<- string]struct{}),
	}
	vm.VmInfo.State = proto.StateStarting
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.vms[ipAddress]; ok {
		return fmt.Errorf("%s already exists", ipAddress)
	}
	for _, poolAddress := range m.addressPool.Registered {
		ipAddr := poolAddress.IpAddress.String()
		if _, ok := requestedIpAddrs[ipAddr]; ok {
			return fmt.Errorf("%s is in address pool", ipAddr)
		}
		if _, ok := requestedMacAddrs[poolAddress.MacAddress]; ok {
			return fmt.Errorf("%s is in address pool", poolAddress.MacAddress)
		}
	}
	subnetId := m.getMatchingSubnet(request.Address.IpAddress)
	if subnetId == "" {
		return fmt.Errorf("no matching subnet for: %s\n", ipAddress)
	}
	vm.VmInfo.SubnetId = subnetId
	vm.VmInfo.SecondarySubnetIDs = nil
	for _, addr := range request.SecondaryAddresses {
		subnetId := m.getMatchingSubnet(addr.IpAddress)
		if subnetId == "" {
			return fmt.Errorf("no matching subnet for: %s\n", addr.IpAddress)
		}
		vm.VmInfo.SecondarySubnetIDs = append(vm.VmInfo.SecondarySubnetIDs,
			subnetId)
	}
	defer func() {
		if vm == nil {
			return
		}
		delete(m.vms, vm.ipAddress)
		m.sendVmInfo(vm.ipAddress, nil)
		os.RemoveAll(vm.dirname)
		for _, volume := range vm.VolumeLocations {
			os.RemoveAll(volume.DirectoryToCleanup)
		}
	}()
	if err := os.MkdirAll(vm.dirname, fsutil.DirPerms); err != nil {
		return err
	}
	for index, sourceFilename := range request.VolumeFilenames {
		dirname := filepath.Join(volumeDirectories[index], ipAddress)
		if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
			return err
		}
		destFilename := filepath.Join(dirname, indexToName(index))
		if err := os.Link(sourceFilename, destFilename); err != nil {
			return err
		}
		vm.VolumeLocations = append(vm.VolumeLocations, proto.LocalVolume{
			dirname, destFilename})
	}
	m.vms[ipAddress] = vm
	if _, err := vm.startManaging(0, false, true); err != nil {
		return err
	}
	vm.setupLockWatcher()
	vm = nil // Cancel cleanup.
	return nil
}

// iterateOverVMs will call the specified function for all VMs matching the
// specified constraints. The Manager read lock will be grabbed and released.
func (m *Manager) iterateOverVMs(ignoreStateMask uint64,
	vmTagsToMatch tags.MatchTags, ownerGroupsList, ownerUsers []string,
	fn func(ipAddr string, vmInfo *vmInfoType)) {
	ownerGroups := stringutil.ConvertListToMap(ownerGroupsList, false)
	vmTagMatcher := tagmatcher.New(vmTagsToMatch, false)
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for ipAddr, vm := range m.vms {
		if ignoreStateMask&(1<<vm.State) != 0 {
			continue
		}
		if !vmTagMatcher.MatchEach(vm.Tags) {
			continue
		}
		include := true
		if len(ownerGroups) > 0 {
			include = false
			for _, ownerGroup := range vm.OwnerGroups {
				if _, ok := ownerGroups[ownerGroup]; ok {
					include = true
					break
				}
			}
		}
		if len(ownerUsers) > 0 {
			include = false
			for _, ownerUser := range ownerUsers {
				if _, ok := vm.ownerUsers[ownerUser]; ok {
					include = true
					break
				}
			}
		}
		if include {
			fn(ipAddr, vm)
		}
	}
}

func (m *Manager) listVMs(request proto.ListVMsRequest) []string {
	var ipAddrs []string
	m.iterateOverVMs(request.IgnoreStateMask, request.VmTagsToMatch,
		request.OwnerGroups, request.OwnerUsers,
		func(ipAddr string, vm *vmInfoType) {
			ipAddrs = append(ipAddrs, ipAddr)
		})
	if request.Sort {
		verstr.Sort(ipAddrs)
	}
	return ipAddrs
}

func (m *Manager) migrateVm(conn *srpc.Conn) error {
	var request proto.MigrateVmRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	hypervisor, err := srpc.DialHTTP("tcp", request.SourceHypervisor, 0)
	if err != nil {
		return err
	}
	defer hypervisor.Close()
	defer func() {
		req := proto.DiscardVmAccessTokenRequest{
			AccessToken: request.AccessToken,
			IpAddress:   request.IpAddress}
		var reply proto.DiscardVmAccessTokenResponse
		hypervisor.RequestReply("Hypervisor.DiscardVmAccessToken",
			req, &reply)
	}()
	ipAddress := request.IpAddress.String()
	m.mutex.RLock()
	_, ok := m.vms[ipAddress]
	subnetId := m.getMatchingSubnet(request.IpAddress)
	m.mutex.RUnlock()
	if ok {
		return errors.New("cannot migrate to the same hypervisor")
	}
	if subnetId == "" {
		return fmt.Errorf("no matching subnet for: %s\n", request.IpAddress)
	}
	getInfoRequest := proto.GetVmInfoRequest{request.IpAddress}
	var getInfoReply proto.GetVmInfoResponse
	err = hypervisor.RequestReply("Hypervisor.GetVmInfo", getInfoRequest,
		&getInfoReply)
	if err != nil {
		return err
	}
	accessToken := request.AccessToken
	vmInfo := getInfoReply.VmInfo
	if subnetId != vmInfo.SubnetId {
		return fmt.Errorf("subnet ID changing from: %s to: %s",
			vmInfo.SubnetId, subnetId)
	}
	if !request.IpAddress.Equal(vmInfo.Address.IpAddress) {
		return fmt.Errorf("inconsistent IP address: %s",
			vmInfo.Address.IpAddress)
	}
	if err := m.migrateVmChecks(vmInfo, request.SkipMemoryCheck); err != nil {
		return err
	}
	volumeDirectories, err := m.getVolumeDirectories(vmInfo.Volumes[0].Size,
		vmInfo.Volumes[0].Type, vmInfo.Volumes[1:], vmInfo.SpreadVolumes, nil)
	if err != nil {
		return err
	}
	vm := &vmInfoType{
		LocalVmInfo: proto.LocalVmInfo{
			VmInfo: vmInfo,
			VolumeLocations: make([]proto.LocalVolume, 0,
				len(volumeDirectories)),
		},
		manager:          m,
		dirname:          filepath.Join(m.StateDir, "VMs", ipAddress),
		doNotWriteOrSend: true,
		ipAddress:        ipAddress,
		logger:           prefixlogger.New(ipAddress+": ", m.Logger),
		metadataChannels: make(map[chan<- string]struct{}),
	}
	vm.Uncommitted = true
	defer func() { // Evaluate vm at return time, not defer time.
		vm.cleanup()
		hyperclient.PrepareVmForMigration(hypervisor, request.IpAddress,
			accessToken, false)
		if vmInfo.State == proto.StateRunning {
			hyperclient.StartVm(hypervisor, request.IpAddress, accessToken)
		}
	}()
	vm.ownerUsers = stringutil.ConvertListToMap(vm.OwnerUsers, false)
	if err := os.MkdirAll(vm.dirname, fsutil.DirPerms); err != nil {
		return err
	}
	for index, _dirname := range volumeDirectories {
		dirname := filepath.Join(_dirname, ipAddress)
		if err := os.MkdirAll(dirname, fsutil.DirPerms); err != nil {
			return err
		}
		vm.VolumeLocations = append(vm.VolumeLocations, proto.LocalVolume{
			DirectoryToCleanup: dirname,
			Filename:           filepath.Join(dirname, indexToName(index)),
		})
	}
	if vmInfo.State == proto.StateStopped {
		err := hyperclient.PrepareVmForMigration(hypervisor, request.IpAddress,
			request.AccessToken, true)
		if err != nil {
			return err
		}
	}
	// Begin copying over the volumes.
	err = sendVmMigrationMessage(conn, "initial volume(s) copy")
	if err != nil {
		return err
	}
	err = vm.migrateVmVolumes(hypervisor, vm.Address.IpAddress, accessToken,
		true)
	if err != nil {
		return err
	}
	if vmInfo.State != proto.StateStopped {
		err = sendVmMigrationMessage(conn, "stopping VM")
		if err != nil {
			return err
		}
		err := hyperclient.StopVm(hypervisor, request.IpAddress,
			request.AccessToken)
		if err != nil {
			return err
		}
		err = hyperclient.PrepareVmForMigration(hypervisor, request.IpAddress,
			request.AccessToken, true)
		if err != nil {
			return err
		}
		err = sendVmMigrationMessage(conn, "update volume(s)")
		if err != nil {
			return err
		}
		err = vm.migrateVmVolumes(hypervisor, vm.Address.IpAddress, accessToken,
			false)
		if err != nil {
			return err
		}
	}
	err = migratevmUserData(hypervisor,
		filepath.Join(vm.dirname, UserDataFile),
		request.IpAddress, accessToken)
	if err != nil {
		return err
	}
	if err := sendVmMigrationMessage(conn, "starting VM"); err != nil {
		return err
	}
	vm.State = proto.StateStarting
	m.mutex.Lock()
	m.vms[ipAddress] = vm
	m.mutex.Unlock()
	dhcpTimedOut, err := vm.startManaging(request.DhcpTimeout, false, false)
	if err != nil {
		return err
	}
	if dhcpTimedOut {
		return fmt.Errorf("DHCP timed out")
	}
	err = conn.Encode(proto.MigrateVmResponse{RequestCommit: true})
	if err != nil {
		return err
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var reply proto.MigrateVmResponseResponse
	if err := conn.Decode(&reply); err != nil {
		return err
	}
	if !reply.Commit {
		return fmt.Errorf("VM migration abandoned")
	}
	if err := m.registerAddress(vm.Address); err != nil {
		return err
	}
	for _, address := range vm.SecondaryAddresses {
		if err := m.registerAddress(address); err != nil {
			return err
		}
	}
	vm.doNotWriteOrSend = false
	vm.Uncommitted = false
	vm.writeAndSendInfo()
	err = hyperclient.DestroyVm(hypervisor, request.IpAddress, accessToken)
	if err != nil {
		m.Logger.Printf("error cleaning up old migrated VM: %s\n", ipAddress)
	}
	vm.setupLockWatcher()
	vm = nil // Cancel cleanup.
	return nil
}

func sendVmCopyMessage(conn *srpc.Conn, message string) error {
	request := proto.CopyVmResponse{ProgressMessage: message}
	if err := conn.Encode(request); err != nil {
		return err
	}
	return conn.Flush()
}

func sendVmMigrationMessage(conn *srpc.Conn, message string) error {
	request := proto.MigrateVmResponse{ProgressMessage: message}
	if err := conn.Encode(request); err != nil {
		return err
	}
	return conn.Flush()
}

func sendVmPatchImageMessage(conn *srpc.Conn, message string) error {
	request := proto.PatchVmImageResponse{ProgressMessage: message}
	if err := conn.Encode(request); err != nil {
		return err
	}
	return conn.Flush()
}

func (m *Manager) migrateVmChecks(vmInfo proto.VmInfo,
	skipMemoryCheck bool) error {
	switch vmInfo.State {
	case proto.StateStopped:
	case proto.StateRunning:
	default:
		return fmt.Errorf("VM state: %s is not stopped/running", vmInfo.State)
	}
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for index, address := range vmInfo.SecondaryAddresses {
		subnetId := m.getMatchingSubnet(address.IpAddress)
		if subnetId == "" {
			return fmt.Errorf("no matching subnet for: %s\n", address.IpAddress)
		}
		if subnetId != vmInfo.SecondarySubnetIDs[index] {
			return fmt.Errorf("subnet ID changing from: %s to: %s",
				vmInfo.SecondarySubnetIDs[index], subnetId)
		}
	}
	if err := m.checkSufficientCPUWithLock(vmInfo.MilliCPUs); err != nil {
		return err
	}
	err := m.checkSufficientMemoryWithLock(vmInfo.MemoryInMiB, nil)
	if err != nil {
		return err
	}
	if !skipMemoryCheck {
		err := <-tryAllocateMemory(getVmInfoMemoryInMiB(vmInfo))
		if err != nil {
			return err
		}
	}
	return nil
}

func migratevmUserData(hypervisor *srpc.Client, filename string,
	ipAddr net.IP, accessToken []byte) error {
	conn, err := hypervisor.Call("Hypervisor.GetVmUserData")
	if err != nil {
		return err
	}
	defer conn.Close()
	request := proto.GetVmUserDataRequest{
		AccessToken: accessToken,
		IpAddress:   ipAddr,
	}
	if err := conn.Encode(request); err != nil {
		return fmt.Errorf("error encoding request: %s", err)
	}
	if err := conn.Flush(); err != nil {
		return err
	}
	var reply proto.GetVmUserDataResponse
	if err := conn.Decode(&reply); err != nil {
		return err
	}
	if err := errors.New(reply.Error); err != nil {
		return err
	}
	if reply.Length < 1 {
		return nil
	}
	writer, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		fsutil.PrivateFilePerms)
	if err != nil {
		io.CopyN(ioutil.Discard, conn, int64(reply.Length))
		return err
	}
	defer writer.Close()
	if _, err := io.CopyN(writer, conn, int64(reply.Length)); err != nil {
		return err
	}
	return nil
}

func (vm *vmInfoType) makeExtraLogger(filename string) (
	*filelogger.Logger, error) {
	debugLevel := int16(-1)
	if levelGetter, ok := vm.logger.(log.DebugLogLevelGetter); ok {
		debugLevel = levelGetter.GetLevel()
	}
	return filelogger.New(filepath.Join(
		vm.VolumeLocations[0].DirectoryToCleanup, filename),
		filelogger.Options{
			Flags:      serverlogger.GetStandardFlags(),
			DebugLevel: debugLevel,
		})
}

func (vm *vmInfoType) migrateVmVolumes(hypervisor *srpc.Client,
	sourceIpAddr net.IP, accessToken []byte, getExtraFiles bool) error {
	for index, volume := range vm.VolumeLocations {
		_, err := migrateVmVolume(hypervisor, volume.DirectoryToCleanup,
			volume.Filename, uint(index), vm.Volumes[index].Size, sourceIpAddr,
			accessToken, getExtraFiles)
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateVmVolume(hypervisor *srpc.Client, directory, filename string,
	volumeIndex uint, size uint64, ipAddr net.IP, accessToken []byte,
	getExtraFiles bool) (
	*rsync.Stats, error) {
	var initialFileSize uint64
	reader, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		defer reader.Close()
		if fi, err := reader.Stat(); err != nil {
			return nil, err
		} else {
			initialFileSize = uint64(fi.Size())
			if initialFileSize > size {
				return nil, errors.New("file larger than volume")
			}
		}
	}
	writer, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE,
		fsutil.PrivateFilePerms)
	if err != nil {
		return nil, err
	}
	defer writer.Close()
	request := proto.GetVmVolumeRequest{
		AccessToken:      accessToken,
		GetExtraFiles:    getExtraFiles,
		IgnoreExtraFiles: !getExtraFiles,
		IpAddress:        ipAddr,
		VolumeIndex:      volumeIndex,
	}
	conn, err := hypervisor.Call("Hypervisor.GetVmVolume")
	if err != nil {
		if reader == nil {
			os.Remove(filename)
		}
		return nil, err
	}
	defer conn.Close()
	if err := conn.Encode(request); err != nil {
		return nil, fmt.Errorf("error encoding request: %s", err)
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}
	var response proto.GetVmVolumeResponse
	if err := conn.Decode(&response); err != nil {
		return nil, err
	}
	if err := errors.New(response.Error); err != nil {
		return nil, err
	}
	stats, err := rsync.GetBlocks(conn, conn, conn, reader, writer, size,
		initialFileSize)
	if err != nil {
		return nil, err
	}
	if !getExtraFiles {
		return &stats, nil
	}
	for name, data := range response.ExtraFiles {
		if name != "initrd" && name != "kernel" {
			return nil, fmt.Errorf("received unsupported extra file: %s", name)
		}
		err := ioutil.WriteFile(filepath.Join(directory, name), data,
			fsutil.PrivateFilePerms)
		if err != nil {
			return nil, err
		}
	}
	return &stats, nil
}

func (m *Manager) notifyVmMetadataRequest(ipAddr net.IP, path string) {
	addr := ipAddr.String()
	m.mutex.RLock()
	vm, ok := m.vms[addr]
	m.mutex.RUnlock()
	if !ok {
		return
	}
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	for ch := range vm.metadataChannels {
		select {
		case ch <- path:
		default:
		}
	}
}

func (m *Manager) patchVmImage(conn *srpc.Conn,
	request proto.PatchVmImageRequest) error {
	client, img, imageName, err := m.getImage(request.ImageName,
		request.ImageTimeout)
	if err != nil {
		return err
	}
	if img.Filter == nil {
		return fmt.Errorf("%s contains no filter", imageName)
	}
	img.FileSystem.InodeToFilenamesTable()
	img.FileSystem.FilenameToInodeTable()
	hashToInodesTable := img.FileSystem.HashToInodesTable()
	img.FileSystem.BuildEntryMap()
	var objectsGetter objectserver.ObjectsGetter
	vm, err := m.getVmLockAndAuth(request.IpAddress, true,
		conn.GetAuthInformation(), nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	haveLock := true
	defer func() {
		vm.allowMutationsAndUnlock(haveLock)
	}()
	restart := vm.State == proto.StateRunning
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if len(vm.Address.IpAddress) < 1 {
			return errors.New("cannot stop VM with externally managed lease")
		}
	default:
		return errors.New("VM is not running or stopped")
	}
	vm.mutex.Unlock()
	haveLock = false
	if m.objectCache == nil {
		objectClient := objclient.AttachObjectClient(client)
		defer objectClient.Close()
		objectsGetter = objectClient
	} else if restart {
		hashes := make([]hash.Hash, 0, len(hashToInodesTable))
		for hashVal := range hashToInodesTable {
			hashes = append(hashes, hashVal)
		}
		if err := sendVmPatchImageMessage(conn, "prefetching"); err != nil {
			return err
		}
		if err := m.objectCache.FetchObjects(hashes); err != nil {
			return err
		}
		objectsGetter = m.objectCache
	} else {
		objectsGetter = m.objectCache
	}
	bootInfo, err := util.GetBootInfo(img.FileSystem, vm.rootLabel(false),
		"net.ifnames=0")
	if err != nil {
		return err
	}
	vm.mutex.Lock()
	haveLock = true
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if err := sendVmPatchImageMessage(conn, "stopping VM"); err != nil {
			return err
		}
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.setState(proto.StateStopping)
		vm.commandInput <- "system_powerdown"
		time.AfterFunc(time.Second*15, vm.kill)
		vm.mutex.Unlock()
		<-stoppedNotifier
		vm.mutex.Lock()
		if vm.State != proto.StateStopped {
			return errors.New("VM is not stopped after stop attempt")
		}
	default:
		return errors.New("VM is not running or stopped")
	}
	vm.mutex.Unlock()
	haveLock = false
	rootFilename := vm.VolumeLocations[0].Filename
	tmpRootFilename := rootFilename + ".new"
	if request.SkipBackup {
		if err := os.Link(rootFilename, tmpRootFilename); err != nil {
			return err
		}
	} else {
		if err := sendVmPatchImageMessage(conn, "copying root"); err != nil {
			return err
		}
		err = fsutil.CopyFile(tmpRootFilename, rootFilename,
			fsutil.PrivateFilePerms)
		if err != nil {
			return err
		}
	}
	defer os.Remove(tmpRootFilename)
	rootDir, err := ioutil.TempDir(vm.dirname, "root")
	if err != nil {
		return err
	}
	defer os.Remove(rootDir)
	partition := "p1"
	loopDevice, err := fsutil.LoopbackSetupAndWaitForPartition(tmpRootFilename,
		partition, time.Minute, vm.logger)
	if err != nil {
		return err
	}
	defer fsutil.LoopbackDeleteAndWaitForPartition(loopDevice, partition,
		time.Minute, vm.logger)
	vm.logger.Debugf(0, "mounting: %s onto: %s\n", loopDevice, rootDir)
	err = wsyscall.Mount(loopDevice+partition, rootDir, "ext4", 0, "")
	if err != nil {
		return err
	}
	defer syscall.Unmount(rootDir, 0)
	if err := sendVmPatchImageMessage(conn, "scanning root"); err != nil {
		return err
	}
	fs, err := scanner.ScanFileSystem(rootDir, nil, img.Filter, nil, nil, nil)
	if err != nil {
		return err
	}
	if err := fs.FileSystem.RebuildInodePointers(); err != nil {
		return err
	}
	fs.FileSystem.BuildEntryMap()
	initrdFilename := vm.getInitrdPath()
	tmpInitrdFilename := initrdFilename + ".new"
	defer os.Remove(tmpInitrdFilename)
	kernelFilename := vm.getKernelPath()
	tmpKernelFilename := kernelFilename + ".new"
	defer os.Remove(tmpKernelFilename)
	writeBootloaderConfig := false
	if _, err := os.Stat(vm.getKernelPath()); err == nil { // No bootloader.
		err := extractKernel(vm.VolumeLocations[0], ".new", objectsGetter,
			img.FileSystem, bootInfo)
		if err != nil {
			return err
		}
	} else { // Requires a bootloader.
		writeBootloaderConfig = true
	}
	patchLogger, err := vm.makeExtraLogger(lastPatchLogFilename)
	if err != nil {
		return err
	}
	defer patchLogger.Close()
	subObj := domlib.Sub{FileSystem: &fs.FileSystem}
	fetchMap, _ := domlib.BuildMissingLists(subObj, img, false, true,
		patchLogger)
	objectsToFetch := objectcache.ObjectMapToCache(fetchMap)
	objectsDir := filepath.Join(rootDir, ".subd", "objects")
	defer os.RemoveAll(objectsDir)
	startTime := time.Now()
	objectsReader, err := objectsGetter.GetObjects(objectsToFetch)
	if err != nil {
		return err
	}
	defer objectsReader.Close()
	err = sendVmPatchImageMessage(conn, "pre-deleting unneeded files")
	if err != nil {
		return err
	}
	err = deleteFilesNotInImage(img.FileSystem, &fs.FileSystem, rootDir,
		patchLogger)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("fetching(%s) %d objects",
		imageName, len(objectsToFetch))
	if err := sendVmPatchImageMessage(conn, msg); err != nil {
		return err
	}
	vm.logger.Debugln(0, msg)
	for _, hashVal := range objectsToFetch {
		length, reader, err := objectsReader.NextObject()
		if err != nil {
			vm.logger.Println(err)
			return err
		}
		err = readOne(objectsDir, hashVal, length, reader)
		reader.Close()
		if err != nil {
			vm.logger.Println(err)
			return err
		}
	}
	msg = fmt.Sprintf("fetched(%s) %d objects in %s",
		imageName, len(objectsToFetch), format.Duration(time.Since(startTime)))
	if err := sendVmPatchImageMessage(conn, msg); err != nil {
		return err
	}
	vm.logger.Debugln(0, msg)
	subObj.ObjectCache = append(subObj.ObjectCache, objectsToFetch...)
	var subRequest subproto.UpdateRequest
	if domlib.BuildUpdateRequest(subObj, img, &subRequest, false, true,
		patchLogger) {
		return errors.New("failed building update: missing computed files")
	}
	subRequest.ImageName = imageName
	subRequest.Triggers = nil
	if err := sendVmPatchImageMessage(conn, "starting update"); err != nil {
		return err
	}
	vm.logger.Debugf(0, "update(%s) starting\n", imageName)
	patchLogger.Printf("update(%s) starting\n", imageName)
	startTime = time.Now()
	_, _, err = sublib.Update(subRequest, rootDir, objectsDir, nil, nil, nil,
		patchLogger)
	if err != nil {
		return err
	}
	msg = fmt.Sprintf("updated(%s) in %s",
		imageName, format.Duration(time.Since(startTime)))
	if err := sendVmPatchImageMessage(conn, msg); err != nil {
		return err
	}
	if writeBootloaderConfig {
		err := bootInfo.WriteBootloaderConfig(rootDir, vm.logger)
		if err != nil {
			return err
		}
	}
	if !request.SkipBackup {
		oldRootFilename := rootFilename + ".old"
		if err := os.Rename(rootFilename, oldRootFilename); err != nil {
			return err
		}
		if err := os.Rename(tmpRootFilename, rootFilename); err != nil {
			os.Rename(oldRootFilename, rootFilename)
			return err
		}
		os.Rename(initrdFilename, initrdFilename+".old")
		os.Rename(kernelFilename, kernelFilename+".old")
	}
	os.Rename(tmpInitrdFilename, initrdFilename)
	os.Rename(tmpKernelFilename, kernelFilename)
	vm.mutex.Lock()
	haveLock = true
	vm.ImageName = imageName
	vm.writeAndSendInfo()
	if restart && vm.State == proto.StateStopped {
		vm.setState(proto.StateStarting)
		sendVmPatchImageMessage(conn, "starting VM")
		vm.mutex.Unlock()
		_, err := vm.startManaging(0, false, false)
		vm.mutex.Lock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) prepareVmForMigration(ipAddr net.IP,
	authInfoP *srpc.AuthInformation, accessToken []byte, enable bool) error {
	authInfo := *authInfoP
	authInfo.HaveMethodAccess = false // Require VM ownership or token.
	vm, err := m.getVmLockAndAuth(ipAddr, true, &authInfo, accessToken)
	if err != nil {
		return nil
	}
	defer vm.mutex.Unlock()
	if enable {
		if vm.Uncommitted {
			return errors.New("VM is uncommitted")
		}
		if vm.State != proto.StateStopped {
			return errors.New("VM is not stopped")
		}
		// Block reallocation of addresses until VM is destroyed, then release
		// claims on addresses.
		vm.Uncommitted = true
		vm.setState(proto.StateMigrating)
		if err := m.unregisterAddress(vm.Address, true); err != nil {
			vm.Uncommitted = false
			vm.setState(proto.StateStopped)
			return err
		}
		for _, address := range vm.SecondaryAddresses {
			if err := m.unregisterAddress(address, true); err != nil {
				vm.logger.Printf("error unregistering address: %s\n",
					address.IpAddress)
				vm.Uncommitted = false
				vm.setState(proto.StateStopped)
				return err
			}
		}
	} else {
		if vm.State != proto.StateMigrating {
			return errors.New("VM is not migrating")
		}
		// Reclaim addresses and then allow reallocation if VM is later
		// destroyed.
		if err := m.registerAddress(vm.Address); err != nil {
			vm.setState(proto.StateStopped)
			return err
		}
		for _, address := range vm.SecondaryAddresses {
			if err := m.registerAddress(address); err != nil {
				vm.logger.Printf("error registering address: %s\n",
					address.IpAddress)
				vm.setState(proto.StateStopped)
				return err
			}
		}
		vm.Uncommitted = false
		vm.setState(proto.StateStopped)
	}
	return nil
}

// rebootVm returns true if the DHCP check timed out.
func (m *Manager) rebootVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	dhcpTimeout time.Duration) (bool, error) {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return false, err
	}
	doUnlock := true
	defer func() {
		if doUnlock {
			vm.mutex.Unlock()
		}
	}()
	switch vm.State {
	case proto.StateStarting:
		return false, errors.New("VM is starting")
	case proto.StateRunning:
		vm.commandInput <- "reboot" // Not a QMP command: interpreted locally.
		vm.mutex.Unlock()
		doUnlock = false
		if dhcpTimeout > 0 {
			ackChan := vm.manager.DhcpServer.MakeAcknowledgmentChannel(
				vm.Address.IpAddress)
			timer := time.NewTimer(dhcpTimeout)
			select {
			case <-ackChan:
				timer.Stop()
			case <-timer.C:
				return true, nil
			}
		}
		return false, nil
	case proto.StateStopping:
		return false, errors.New("VM is stopping")
	case proto.StateStopped:
		return false, errors.New("VM is stopped")
	case proto.StateFailedToStart:
		return false, errors.New("VM failed to start")
	case proto.StateExporting:
		return false, errors.New("VM is exporting")
	case proto.StateCrashed:
		return false, errors.New("VM has crashed")
	case proto.StateDestroying:
		return false, errors.New("VM is destroying")
	case proto.StateMigrating:
		return false, errors.New("VM is migrating")
	case proto.StateDebugging:
		return false, errors.New("VM is debugging")
	default:
		return false, errors.New("unknown state: " + vm.State.String())
	}
}

func (m *Manager) registerVmMetadataNotifier(ipAddr net.IP,
	authInfo *srpc.AuthInformation, pathChannel chan<- string) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	vm.metadataChannels[pathChannel] = struct{}{}
	return nil
}

func (m *Manager) replaceVmImage(conn *srpc.Conn,
	authInfo *srpc.AuthInformation) error {

	sendError := func(conn *srpc.Conn, err error) error {
		return conn.Encode(proto.ReplaceVmImageResponse{Error: err.Error()})
	}

	sendUpdate := func(conn *srpc.Conn, message string) error {
		response := proto.ReplaceVmImageResponse{ProgressMessage: message}
		if err := conn.Encode(response); err != nil {
			return err
		}
		return conn.Flush()
	}

	var request proto.ReplaceVmImageRequest
	if err := conn.Decode(&request); err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(request.IpAddress, true, authInfo, nil)
	if err != nil {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	vm.blockMutations = true
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if len(vm.Address.IpAddress) < 1 {
			err = errors.New("cannot stop VM with externally managed lease")
		}
	default:
		err = errors.New("VM is not running or stopped")
	}
	if err != nil {
		vm.allowMutationsAndUnlock(true)
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		return sendError(conn, err)
	}
	restart := vm.State == proto.StateRunning
	vm.mutex.Unlock()
	haveLock := false
	defer func() {
		vm.allowMutationsAndUnlock(haveLock)
	}()
	initrdFilename := vm.getInitrdPath()
	tmpInitrdFilename := initrdFilename + ".new"
	defer os.Remove(tmpInitrdFilename)
	kernelFilename := vm.getKernelPath()
	tmpKernelFilename := kernelFilename + ".new"
	defer os.Remove(tmpKernelFilename)
	tmpRootFilename := vm.VolumeLocations[0].Filename + ".new"
	defer os.Remove(tmpRootFilename)
	var newSize uint64
	if request.ImageName != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		if err := sendUpdate(conn, "getting image"); err != nil {
			return err
		}
		client, img, imageName, err := m.getImage(request.ImageName,
			request.ImageTimeout)
		if err != nil {
			return sendError(conn, err)
		}
		defer client.Close()
		request.ImageName = imageName
		err = sendUpdate(conn, "unpacking image: "+imageName)
		if err != nil {
			return err
		}
		writeRawOptions := util.WriteRawOptions{
			ExtraKernelOptions: vm.ExtraKernelOptions,
			InitialImageName:   imageName,
			MinimumFreeBytes:   request.MinimumFreeBytes,
			OverlayFiles:       request.OverlayFiles,
			RootLabel:          vm.rootLabelSaved(false),
			RoundupPower:       request.RoundupPower,
		}
		err = m.writeRaw(vm.VolumeLocations[0], ".new", client, img.FileSystem,
			writeRawOptions, request.SkipBootloader)
		if err != nil {
			return sendError(conn, err)
		}
		if fi, err := os.Stat(tmpRootFilename); err != nil {
			return sendError(conn, err)
		} else {
			newSize = uint64(fi.Size())
		}
	} else if request.ImageDataSize > 0 {
		err := copyData(tmpRootFilename, conn, request.ImageDataSize, m.DisableFillZero, vm.logger)
		if err != nil {
			return err
		}
		newSize = computeSize(request.MinimumFreeBytes, request.RoundupPower,
			request.ImageDataSize)
		if err := setVolumeSize(tmpRootFilename, newSize); err != nil {
			return sendError(conn, err)
		}
	} else if request.ImageURL != "" {
		if err := maybeDrainImage(conn, request.ImageDataSize); err != nil {
			return err
		}
		httpResponse, err := http.Get(request.ImageURL)
		if err != nil {
			return sendError(conn, err)
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode != http.StatusOK {
			return sendError(conn, errors.New(httpResponse.Status))
		}
		if httpResponse.ContentLength < 0 {
			return sendError(conn,
				errors.New("ContentLength from: "+request.ImageURL))
		}
		err = copyData(tmpRootFilename, httpResponse.Body,
			uint64(httpResponse.ContentLength), m.DisableFillZero, vm.logger)
		if err != nil {
			return sendError(conn, err)
		}
		newSize = computeSize(request.MinimumFreeBytes, request.RoundupPower,
			uint64(httpResponse.ContentLength))
		if err := setVolumeSize(tmpRootFilename, newSize); err != nil {
			return sendError(conn, err)
		}
	} else {
		return sendError(conn, errors.New("no image specified"))
	}
	vm.mutex.Lock()
	haveLock = true
	switch vm.State {
	case proto.StateStopped:
	case proto.StateRunning:
		if err := sendUpdate(conn, "stopping VM"); err != nil {
			return err
		}
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.setState(proto.StateStopping)
		vm.commandInput <- "system_powerdown"
		time.AfterFunc(time.Second*15, vm.kill)
		vm.mutex.Unlock()
		<-stoppedNotifier
		vm.mutex.Lock()
		if vm.State != proto.StateStopped {
			return sendError(conn,
				errors.New("VM is not stopped after stop attempt"))
		}
	default:
		return sendError(conn, errors.New("VM is not running or stopped"))
	}
	rootFilename := vm.VolumeLocations[0].Filename
	if request.SkipBackup {
		if err := os.Rename(tmpRootFilename, rootFilename); err != nil {
			return sendError(conn, err)
		}
	} else {
		oldRootFilename := vm.VolumeLocations[0].Filename + ".old"
		if err := os.Rename(rootFilename, oldRootFilename); err != nil {
			return sendError(conn, err)
		}
		if err := os.Rename(tmpRootFilename, rootFilename); err != nil {
			os.Rename(oldRootFilename, rootFilename)
			return sendError(conn, err)
		}
		os.Rename(initrdFilename, initrdFilename+".old")
		os.Rename(kernelFilename, kernelFilename+".old")
	}
	os.Rename(tmpInitrdFilename, initrdFilename)
	os.Rename(tmpKernelFilename, kernelFilename)
	if request.ImageName != "" {
		vm.ImageName = request.ImageName
	}
	vm.Volumes[0].Size = newSize
	vm.writeAndSendInfo()
	if restart && vm.State == proto.StateStopped {
		vm.setState(proto.StateStarting)
		sendUpdate(conn, "starting VM")
		vm.mutex.Unlock()
		_, err := vm.startManaging(0, false, false)
		vm.mutex.Lock()
		if err != nil {
			sendError(conn, err)
		}
	}
	response := proto.ReplaceVmImageResponse{
		Final: true,
	}
	if err := conn.Encode(response); err != nil {
		return err
	}
	return nil
}

func (m *Manager) replaceVmUserData(ipAddr net.IP, reader io.Reader,
	size uint64, authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	filename := filepath.Join(vm.dirname, UserDataFile)
	oldFilename := filename + ".old"
	newFilename := filename + ".new"
	err = fsutil.CopyToFile(newFilename, fsutil.PrivateFilePerms, reader, size)
	if err != nil {
		return err
	}
	defer os.Remove(newFilename)
	if err := os.Rename(filename, oldFilename); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(newFilename, filename); err != nil {
		os.Rename(oldFilename, filename)
		return err
	}
	return nil
}

func (m *Manager) restoreVmFromSnapshot(ipAddr net.IP,
	authInfo *srpc.AuthInformation, forceIfNotStopped bool,
	snapshotName string) error {
	snapshotSuffix, err := sanitiseSnapshotName(snapshotName)
	if err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	var changed bool
	defer func() {
		if changed {
			vm.writeAndSendInfo()
		}
	}()
	if vm.State != proto.StateStopped {
		if !forceIfNotStopped {
			return errors.New("VM is not stopped")
		}
	}
	for index, volume := range vm.VolumeLocations {
		snapshotFilename := volume.Filename + "." + snapshotSuffix
		if err := os.Rename(snapshotFilename, volume.Filename); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
		vm.mutex.Lock()
		delete(vm.Volumes[index].Snapshots, snapshotName)
		vm.mutex.Unlock()
		changed = true
	}
	return nil
}

func (m *Manager) restoreVmImage(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	rootFilename := vm.VolumeLocations[0].Filename
	oldRootFilename := vm.VolumeLocations[0].Filename + ".old"
	fi, err := os.Stat(oldRootFilename)
	if err != nil {
		return err
	}
	if err := os.Rename(oldRootFilename, rootFilename); err != nil {
		return err
	}
	initrdFilename := vm.getInitrdPath()
	os.Rename(initrdFilename+".old", initrdFilename)
	kernelFilename := vm.getKernelPath()
	os.Rename(kernelFilename+".old", kernelFilename)
	vm.Volumes[0].Size = uint64(fi.Size())
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) restoreVmUserData(ipAddr net.IP,
	authInfo *srpc.AuthInformation) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	filename := filepath.Join(vm.dirname, UserDataFile)
	oldFilename := filename + ".old"
	return os.Rename(oldFilename, filename)
}

func (m *Manager) reorderVmVolumes(ipAddr net.IP,
	authInfo *srpc.AuthInformation, accessToken []byte,
	_volumeIndices []uint) error {
	// If root volume isn't listed, insert default "keep in place" entry.
	var volumeIndices []uint
	for _, oldIndex := range _volumeIndices {
		if oldIndex == 0 {
			volumeIndices = _volumeIndices
			break
		}
	}
	if volumeIndices == nil {
		volumeIndices = make([]uint, 1) // Map 0->0.
		volumeIndices = append(volumeIndices, _volumeIndices...)
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	if volumeIndices[0] != 0 {
		if vm.getActiveInitrdPath() != "" {
			return errors.New("cannot reorder root volume with separate initrd")
		}
		if vm.getActiveKernelPath() != "" {
			return errors.New("cannot reorder root volume with separate kernel")
		}
	}
	if len(volumeIndices) != len(vm.VolumeLocations) {
		return fmt.Errorf(
			"number of volume indices: %d != number of volumes: %d",
			len(volumeIndices), len(vm.VolumeLocations))
	}
	if vm.State != proto.StateStopped {
		return errors.New("VM is not stopped")
	}
	var pathsToRename []string
	defer func() {
		for _, path := range pathsToRename {
			os.Remove(path + "~")
		}
	}()
	indexMap := make(map[uint]struct{}, len(volumeIndices))
	volumeLocations := make([]proto.LocalVolume, len(volumeIndices))
	volumes := make([]proto.Volume, len(volumeIndices))
	for newIndex, oldIndex := range volumeIndices {
		if oldIndex >= uint(len(vm.VolumeLocations)) {
			return fmt.Errorf("volume index: %d too large", oldIndex)
		}
		if _, ok := indexMap[oldIndex]; ok {
			return fmt.Errorf("duplicate volume index: %d", oldIndex)
		}
		indexMap[oldIndex] = struct{}{}
		vl := vm.VolumeLocations[oldIndex]
		if newIndex != int(oldIndex) {
			newName := filepath.Join(vl.DirectoryToCleanup,
				indexToName(newIndex))
			if err := os.Link(vl.Filename, newName+"~"); err != nil {
				return err
			}
			pathsToRename = append(pathsToRename, newName)
			vl.Filename = newName
		}
		volumeLocations[newIndex] = vl
		volumes[newIndex] = vm.Volumes[oldIndex]
	}
	for _, path := range pathsToRename {
		os.Rename(path+"~", path)
	}
	pathsToRename = nil
	vm.VolumeLocations = volumeLocations
	vm.Volumes = volumes
	vm.writeAndSendInfo()
	return nil
}

func (m *Manager) scanVmRoot(ipAddr net.IP, authInfo *srpc.AuthInformation,
	scanFilter *filter.Filter) (*filesystem.FileSystem, error) {
	vm, err := m.getVmLockAndAuth(ipAddr, false, authInfo, nil)
	if err != nil {
		return nil, err
	}
	defer vm.mutex.RUnlock()
	return vm.scanVmRoot(scanFilter)
}

func (vm *vmInfoType) scanVmRoot(scanFilter *filter.Filter) (
	*filesystem.FileSystem, error) {
	if vm.State != proto.StateStopped {
		return nil, errors.New("VM is not stopped")
	}
	rootDir, err := ioutil.TempDir(vm.dirname, "root")
	if err != nil {
		return nil, err
	}
	defer os.Remove(rootDir)
	partition := "p1"
	loopDevice, err := fsutil.LoopbackSetupAndWaitForPartition(
		vm.VolumeLocations[0].Filename, partition, time.Minute, vm.logger)
	if err != nil {
		return nil, err
	}
	defer fsutil.LoopbackDeleteAndWaitForPartition(loopDevice, partition,
		time.Minute, vm.logger)
	blockDevice := loopDevice + partition
	vm.logger.Debugf(0, "mounting: %s onto: %s\n", blockDevice, rootDir)
	err = wsyscall.Mount(blockDevice, rootDir, "ext4", 0, "")
	if err != nil {
		return nil, fmt.Errorf("error mounting: %s: %s", blockDevice, err)
	}
	defer syscall.Unmount(rootDir, 0)
	sfs, err := scanner.ScanFileSystem(rootDir, nil, scanFilter, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return &sfs.FileSystem, nil
}

func (m *Manager) sendVmInfo(ipAddress string, vm *proto.VmInfo) {
	if ipAddress != "0.0.0.0" {
		if vm == nil { // GOB cannot encode a nil value in a map.
			vm = new(proto.VmInfo)
		}
		m.sendUpdate(proto.Update{
			HaveVMs: true,
			VMs:     map[string]*proto.VmInfo{ipAddress: vm},
		})
	}
}

func (m *Manager) snapshotVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	forceIfNotStopped, snapshotRootOnly bool, snapshotName string) error {
	snapshotSuffix, err := sanitiseSnapshotName(snapshotName)
	if err != nil {
		return err
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, nil)
	if err != nil {
		return err
	}
	vm.blockMutations = true
	vm.mutex.Unlock()
	defer vm.allowMutationsAndUnlock(false)
	if vm.getActiveInitrdPath() != "" {
		return errors.New("cannot snapshot root volume with separate initrd")
	}
	if vm.getActiveKernelPath() != "" {
		return errors.New("cannot snapshot root volume with separate kernel")
	}
	// TODO(rgooch): First check for sufficient free space.
	if vm.State != proto.StateStopped {
		if !forceIfNotStopped {
			return errors.New("VM is not stopped")
		}
	}
	changed, err := vm.discardSnapshot(snapshotName, snapshotSuffix)
	if err != nil {
		if changed {
			vm.writeAndSendInfo()
		}
		return err
	}
	doCleanup := true
	defer func() {
		if doCleanup {
			if cng, _ := vm.discardSnapshot(snapshotName, snapshotSuffix); cng {
				changed = true
			}
		}
		if changed {
			vm.writeAndSendInfo()
		}
	}()
	for index, volume := range vm.VolumeLocations {
		snapshotFilename := volume.Filename + "." + snapshotSuffix
		if index == 0 || !snapshotRootOnly {
			err := fsutil.CopyFile(snapshotFilename, volume.Filename,
				fsutil.PrivateFilePerms)
			if err != nil {
				return err
			}
			fi, err := os.Stat(snapshotFilename)
			if err != nil {
				return fmt.Errorf("cannot stat: %s: %s", snapshotFilename, err)
			}
			vm.mutex.Lock()
			if vm.Volumes[index].Snapshots == nil {
				vm.Volumes[index].Snapshots = make(map[string]uint64)
			}
			vm.Volumes[index].Snapshots[snapshotName] = uint64(fi.Size())
			vm.mutex.Unlock()
			changed = true
		}
	}
	doCleanup = false
	return nil
}

// startVm returns true if the DHCP check timed out.
func (m *Manager) startVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte, dhcpTimeout time.Duration) (bool, error) {
	if m.disabled {
		return false, errors.New("Hypervisor is disabled")
	}
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return false, err
	}
	doUnlock := true
	defer func() {
		if doUnlock {
			vm.mutex.Unlock()
		}
	}()
	if err := checkAvailableMemory(vm.MemoryInMiB); err != nil {
		return false, err
	}
	switch vm.State {
	case proto.StateStarting:
		return false, errors.New("VM is already starting")
	case proto.StateRunning:
		return false, errors.New("VM is running")
	case proto.StateStopping:
		return false, errors.New("VM is stopping")
	case proto.StateStopped, proto.StateFailedToStart, proto.StateExporting,
		proto.StateCrashed:
		vm.setState(proto.StateStarting)
		vm.mutex.Unlock()
		doUnlock = false
		return vm.startManaging(dhcpTimeout, false, false)
	case proto.StateDestroying:
		return false, errors.New("VM is destroying")
	case proto.StateMigrating:
		return false, errors.New("VM is migrating")
	case proto.StateDebugging:
		debugRoot := vm.getDebugRoot()
		if debugRoot == "" {
			return false, errors.New("debugging volume missing")
		}
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.setState(proto.StateStopping)
		vm.commandInput <- "system_powerdown"
		time.AfterFunc(time.Second*15, vm.kill)
		vm.mutex.Unlock()
		<-stoppedNotifier
		vm.mutex.Lock()
		if vm.State != proto.StateStopped {
			return false, errors.New("VM is not stopped after stop attempt")
		}
		if err := os.Remove(debugRoot); err != nil {
			return false, err
		}
		vm.writeAndSendInfo()
		vm.setState(proto.StateStarting)
		vm.mutex.Unlock()
		doUnlock = false
		return vm.startManaging(dhcpTimeout, false, false)
	default:
		return false, errors.New("unknown state: " + vm.State.String())
	}
}

func (m *Manager) stopVm(ipAddr net.IP, authInfo *srpc.AuthInformation,
	accessToken []byte) error {
	vm, err := m.getVmLockAndAuth(ipAddr, true, authInfo, accessToken)
	if err != nil {
		return err
	}
	doUnlock := true
	defer func() {
		if doUnlock {
			vm.mutex.Unlock()
		}
	}()
	switch vm.State {
	case proto.StateStarting:
		return errors.New("VM is starting")
	case proto.StateRunning, proto.StateDebugging:
		if len(vm.Address.IpAddress) < 1 {
			return errors.New("cannot stop VM with externally managed lease")
		}
		if debugRoot := vm.getDebugRoot(); debugRoot != "" {
			if err := os.Remove(debugRoot); err != nil {
				return err
			}
		}
		stoppedNotifier := make(chan struct{}, 1)
		vm.stoppedNotifier = stoppedNotifier
		vm.setState(proto.StateStopping)
		vm.commandInput <- "system_powerdown"
		time.AfterFunc(time.Second*15, vm.kill)
		vm.mutex.Unlock()
		doUnlock = false
		<-stoppedNotifier
	case proto.StateFailedToStart:
		vm.setState(proto.StateStopped)
	case proto.StateStopping:
		return errors.New("VM is stopping")
	case proto.StateStopped:
		return errors.New("VM is already stopped")
	case proto.StateDestroying:
		return errors.New("VM is destroying")
	case proto.StateMigrating:
		return errors.New("VM is migrating")
	case proto.StateExporting:
		return errors.New("VM is exporting")
	case proto.StateCrashed:
		vm.setState(proto.StateStopped)
	default:
		return errors.New("unknown state: " + vm.State.String())
	}
	return nil
}

func (m *Manager) unregisterVmMetadataNotifier(ipAddr net.IP,
	pathChannel chan<- string) error {
	vm, err := m.getVmAndLock(ipAddr, true)
	if err != nil {
		return err
	}
	defer vm.mutex.Unlock()
	delete(vm.metadataChannels, pathChannel)
	return nil
}

func (m *Manager) writeRaw(volume proto.LocalVolume, extension string,
	client *srpc.Client, fs *filesystem.FileSystem,
	writeRawOptions util.WriteRawOptions, skipBootloader bool) error {
	startTime := time.Now()
	var objectsGetter objectserver.ObjectsGetter
	if m.objectCache == nil {
		objectClient := objclient.AttachObjectClient(client)
		defer objectClient.Close()
		objectsGetter = objectClient
	} else {
		objectsGetter = m.objectCache
	}
	writeRawOptions.AllocateBlocks = true
	if skipBootloader {
		bootInfo, err := util.GetBootInfo(fs, writeRawOptions.RootLabel, "")
		if err != nil {
			return err
		}
		err = extractKernel(volume, extension, objectsGetter, fs, bootInfo)
		if err != nil {
			return err
		}
	} else {
		writeRawOptions.InstallBootloader = true
	}
	writeRawOptions.WriteFstab = true
	err := util.WriteRawWithOptions(fs, objectsGetter,
		volume.Filename+extension, fsutil.PrivateFilePerms,
		mbr.TABLE_TYPE_MSDOS, writeRawOptions, m.Logger)
	if err != nil {
		return err
	}
	m.Logger.Debugf(1, "Wrote root volume in %s\n",
		format.Duration(time.Since(startTime)))
	return nil
}

func (vm *vmInfoType) autoDestroy() {
	vm.logger.Println("VM was not acknowledged, destroying")
	authInfo := &srpc.AuthInformation{HaveMethodAccess: true}
	err := vm.manager.destroyVm(vm.Address.IpAddress, authInfo, nil)
	if err != nil {
		vm.logger.Println(err)
	}
}

func (vm *vmInfoType) changeIpAddress(ipAddress string) error {
	dirname := filepath.Join(vm.manager.StateDir, "VMs", ipAddress)
	if err := os.Rename(vm.dirname, dirname); err != nil {
		return err
	}
	vm.dirname = dirname
	for index, volume := range vm.VolumeLocations {
		parent := filepath.Dir(volume.DirectoryToCleanup)
		dirname := filepath.Join(parent, ipAddress)
		if err := os.Rename(volume.DirectoryToCleanup, dirname); err != nil {
			return err
		}
		vm.VolumeLocations[index] = proto.LocalVolume{
			DirectoryToCleanup: dirname,
			Filename: filepath.Join(dirname,
				filepath.Base(volume.Filename)),
		}
	}
	vm.logger.Printf("changing to new address: %s\n", ipAddress)
	vm.logger = prefixlogger.New(ipAddress+": ", vm.manager.Logger)
	vm.writeInfo()
	vm.manager.mutex.Lock()
	defer vm.manager.mutex.Unlock()
	delete(vm.manager.vms, vm.ipAddress)
	vm.ipAddress = ipAddress
	vm.manager.vms[vm.ipAddress] = vm
	vm.manager.sendUpdate(proto.Update{
		HaveVMs: true,
		VMs:     map[string]*proto.VmInfo{ipAddress: &vm.VmInfo},
	})
	return nil
}

func (vm *vmInfoType) checkAuth(authInfo *srpc.AuthInformation,
	accessToken []byte) error {
	if authInfo.HaveMethodAccess {
		return nil
	}
	if _, ok := vm.ownerUsers[authInfo.Username]; ok {
		return nil
	}
	if len(vm.accessToken) >= 32 && bytes.Equal(vm.accessToken, accessToken) {
		return nil
	}
	for _, ownerGroup := range vm.OwnerGroups {
		if _, ok := authInfo.GroupList[ownerGroup]; ok {
			return nil
		}
	}
	return errorNoAccessToResource
}

func (vm *vmInfoType) cleanup() {
	if vm == nil {
		return
	}
	select {
	case vm.commandInput <- "quit":
	default:
	}
	m := vm.manager
	m.mutex.Lock()
	delete(m.vms, vm.ipAddress)
	if !vm.doNotWriteOrSend {
		m.sendVmInfo(vm.ipAddress, nil)
	}
	if !vm.Uncommitted {
		if err := m.releaseAddressInPoolWithLock(vm.Address); err != nil {
			m.Logger.Println(err)
		}
		for _, address := range vm.SecondaryAddresses {
			if err := m.releaseAddressInPoolWithLock(address); err != nil {
				m.Logger.Println(err)
			}
		}
	}
	os.RemoveAll(vm.dirname)
	for _, volume := range vm.VolumeLocations {
		os.RemoveAll(volume.DirectoryToCleanup)
	}
	m.mutex.Unlock()
}

func (vm *vmInfoType) copyRootVolume(request proto.CreateVmRequest,
	reader io.Reader, dataSize uint64, volumeType proto.VolumeType) error {
	err := vm.setupVolumes(dataSize, volumeType, request.SecondaryVolumes,
		request.SpreadVolumes, nil)
	if err != nil {
		return err
	}
	err = copyData(vm.VolumeLocations[0].Filename, reader, dataSize, false, vm.logger)
	if err != nil {
		return err
	}
	var format proto.VolumeFormat
	if len(request.Volumes) > 0 {
		format = request.Volumes[0].Format
	}
	vm.Volumes = []proto.Volume{{Format: format, Size: dataSize}}
	return nil
}

// delete deletes external VM state (files, leases, IPs). The VM lock will be
// released and later grabbed. The Manager lock will be grabbed and released
// while the VM lock is not held.
func (vm *vmInfoType) delete() {
	vm.logger.Debugln(2, "delete(): starting")
	vm.State = proto.StateDestroying
	select {
	case vm.accessTokenCleanupNotifier <- struct{}{}:
	default:
	}
	for ch := range vm.metadataChannels {
		close(ch)
	}
	if vm.identityProviderNotifier != nil {
		close(vm.identityProviderNotifier)
		vm.identityProviderNotifier = nil
	}
	vm.mutex.Unlock()
	for _, volume := range vm.VolumeLocations {
		os.Remove(volume.Filename)
		if volume.DirectoryToCleanup != "" {
			os.RemoveAll(volume.DirectoryToCleanup)
		}
	}
	os.RemoveAll(vm.dirname)
	vm.manager.DhcpServer.RemoveLease(vm.Address.IpAddress)
	for _, address := range vm.SecondaryAddresses {
		vm.manager.DhcpServer.RemoveLease(address.IpAddress)
	}
	vm.manager.mutex.Lock()
	delete(vm.manager.vms, vm.ipAddress)
	var err error
	if vm.State == proto.StateExporting {
		err = vm.manager.unregisterAddress(vm.Address, false)
		for _, address := range vm.SecondaryAddresses {
			err := vm.manager.unregisterAddress(address, false)
			if err != nil {
				vm.manager.Logger.Println(err)
			}
		}
	} else if !vm.Uncommitted {
		err = vm.manager.releaseAddressInPoolWithLock(vm.Address)
		for _, address := range vm.SecondaryAddresses {
			err := vm.manager.releaseAddressInPoolWithLock(address)
			if err != nil {
				vm.manager.Logger.Println(err)
			}
		}
	}
	vm.manager.mutex.Unlock()
	if err != nil {
		vm.manager.Logger.Println(err)
	}
	vm.manager.sendVmInfo(vm.ipAddress, nil) // Send now that VM is really gone.
	if vm.lockWatcher != nil {
		vm.lockWatcher.Stop()
	}
	vm.mutex.Lock()
	vm.logger.Debugln(2, "delete(): returning")
}

func (vm *vmInfoType) destroy() {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	select {
	case vm.commandInput <- "quit":
	default:
	}
	vm.delete()
}

// discardSnapshot will delete the specified snapshot. The VM lock will be
// grabbed and released.
// true is returned if the VM data structure is modified.
func (vm *vmInfoType) discardSnapshot(snapshotName, snapshotSuff string) (
	bool, error) {
	var changed bool
	for index, volume := range vm.VolumeLocations {
		if err := removeFile(volume.Filename + "." + snapshotSuff); err != nil {
			return changed, err
		}
		vm.mutex.Lock()
		delete(vm.Volumes[index].Snapshots, snapshotName)
		vm.mutex.Unlock()
		changed = true
	}
	return changed, nil
}

func (vm *vmInfoType) getActiveInitrdPath() string {
	initrdPath := vm.getInitrdPath()
	if _, err := os.Stat(initrdPath); err == nil {
		return initrdPath
	}
	return ""
}

func (vm *vmInfoType) getActiveKernelPath() string {
	kernelPath := vm.getKernelPath()
	if _, err := os.Stat(kernelPath); err == nil {
		return kernelPath
	}
	return ""
}

func (vm *vmInfoType) getDebugRoot() string {
	filename := vm.VolumeLocations[0].Filename + ".debug"
	if _, err := os.Stat(filename); err == nil {
		return filename
	}
	return ""
}

func (vm *vmInfoType) getInitrdPath() string {
	return filepath.Join(vm.VolumeLocations[0].DirectoryToCleanup, "initrd")
}

func (vm *vmInfoType) getKernelPath() string {
	return filepath.Join(vm.VolumeLocations[0].DirectoryToCleanup, "kernel")
}

func (vm *vmInfoType) kill() {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()
	if vm.State == proto.StateStopping {
		vm.commandInput <- "quit"
	}
}

func (vm *vmInfoType) monitor(monitorSock net.Conn,
	commandInput <-chan string, commandOutput chan<- byte) {
	vm.hasHealthAgent = false
	defer monitorSock.Close()
	go vm.processMonitorResponses(monitorSock, commandOutput)
	cancelChannel := make(chan struct{}, 1)
	go vm.probeHealthAgent(cancelChannel)
	go vm.serialManager()
	for command := range commandInput {
		var err error
		if command == "reboot" { // Not a QMP command: convert to ctrl-alt-del.
			_, err = monitorSock.Write([]byte(rebootJson))
		} else if command[0] == '\\' {
			_, err = fmt.Fprintln(monitorSock, command[1:])
		} else {
			_, err = fmt.Fprintf(monitorSock, "{\"execute\":\"%s\"}\n",
				command)
		}
		if err != nil {
			vm.logger.Println(err)
		} else if command[0] == '\\' {
			vm.logger.Debugf(0, "sent JSON: %s", command[1:])
		} else {
			vm.logger.Debugf(0, "sent %s command", command)
		}
	}
	cancelChannel <- struct{}{}
}

func (vm *vmInfoType) probeHealthAgent(cancel <-chan struct{}) {
	stopTime := time.Now().Add(time.Minute * 5)
	for time.Until(stopTime) > 0 {
		select {
		case <-cancel:
			return
		default:
		}
		sleepUntil := time.Now().Add(time.Second)
		if vm.ipAddress == "0.0.0.0" {
			time.Sleep(time.Until(sleepUntil))
			continue
		}
		conn, err := net.DialTimeout("tcp", vm.ipAddress+":6910", time.Second*5)
		if err == nil {
			conn.Close()
			vm.mutex.Lock()
			vm.hasHealthAgent = true
			vm.mutex.Unlock()
			return
		}
		time.Sleep(time.Until(sleepUntil))
	}
}

func (vm *vmInfoType) rootLabel(debug bool) string {
	ipAddr := vm.Address.IpAddress
	var prefix string
	if debug {
		prefix = "debugfs" // 16 characters: the limit.
	} else {
		prefix = "rootfs"
	}
	return fmt.Sprintf("%s@%02x%02x%02x%02x",
		prefix, ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3])
}

func (vm *vmInfoType) rootLabelSaved(debug bool) string {
	ipAddr := vm.Address.IpAddress
	var prefix string
	if debug {
		prefix = "debugfs" // 16 characters: the limit.
	} else {
		if vm.RootFileSystemLabel != "" {
			return vm.RootFileSystemLabel
		}
		prefix = "rootfs"
	}
	return fmt.Sprintf("%s@%02x%02x%02x%02x",
		prefix, ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3])
}

func (vm *vmInfoType) serialManager() {
	bootlogFile, err := os.OpenFile(filepath.Join(vm.dirname, bootlogFilename),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, fsutil.PublicFilePerms)
	if err != nil {
		vm.logger.Printf("error opening bootlog file: %s\n", err)
		return
	}
	defer bootlogFile.Close()
	serialSock, err := net.Dial("unix",
		filepath.Join(vm.dirname, serialSockFilename))
	if err != nil {
		vm.logger.Printf("error connecting to console: %s\n", err)
		return
	}
	defer serialSock.Close()
	vm.mutex.Lock()
	vm.serialInput = serialSock
	vm.mutex.Unlock()
	buffer := make([]byte, 256)
	for {
		if nRead, err := serialSock.Read(buffer); err != nil {
			if err != io.EOF {
				vm.logger.Printf("error reading from serial port: %s\n", err)
			} else {
				vm.logger.Debugln(0, "serial port closed")
			}
			break
		} else if nRead > 0 {
			vm.mutex.RLock()
			if vm.serialOutput != nil {
				for _, char := range buffer[:nRead] {
					vm.serialOutput <- char
				}
				vm.mutex.RUnlock()
			} else {
				vm.mutex.RUnlock()
				bootlogFile.Write(buffer[:nRead])
			}
		}
	}
	vm.mutex.Lock()
	vm.serialInput = nil
	if vm.serialOutput != nil {
		close(vm.serialOutput)
		vm.serialOutput = nil
	}
	vm.mutex.Unlock()
}

func (vm *vmInfoType) setState(state proto.State) {
	if state != vm.State {
		vm.ChangedStateOn = time.Now()
		vm.State = state
	}
	if !vm.doNotWriteOrSend {
		vm.writeAndSendInfo()
	}
}

// This may grab and release the VM lock.
// If dhcpTimeout <0: no DHCP lease is set up, if 0, do not wait for DHCP ACK,
// else wait for DHCP ACK.
// It returns true if there was a timeout waiting for the DHCP request, else
// false.
func (vm *vmInfoType) startManaging(dhcpTimeout time.Duration,
	enableNetboot, haveManagerLock bool) (bool, error) {
	vm.monitorSockname = filepath.Join(vm.dirname, "monitor.sock")
	vm.logger.Debugln(1, "startManaging() starting")
	switch vm.State {
	case proto.StateStarting:
	case proto.StateRunning:
	case proto.StateFailedToStart:
	case proto.StateStopping:
		monitorSock, err := net.Dial("unix", vm.monitorSockname)
		if err == nil {
			commandInput := make(chan string, 2)
			commandOutput := make(chan byte, 16<<10)
			vm.commandInput = commandInput
			vm.commandOutput = commandOutput
			go vm.monitor(monitorSock, commandInput, commandOutput)
			commandInput <- "qmp_capabilities"
			commandInput <- "quit"
		} else {
			vm.setState(proto.StateStopped)
			vm.logger.Println(err)
		}
		return false, nil
	case proto.StateStopped:
		return false, nil
	case proto.StateDestroying:
		vm.mutex.Lock()
		vm.delete()
		vm.mutex.Unlock()
		return false, nil
	case proto.StateMigrating:
		return false, nil
	case proto.StateCrashed:
	case proto.StateDebugging:
	default:
		vm.logger.Println("unknown state: " + vm.State.String())
		return false, nil
	}
	if err := vm.checkVolumes(true); err != nil {
		vm.setState(proto.StateFailedToStart)
		return false, err
	}
	if dhcpTimeout >= 0 {
		err := vm.manager.DhcpServer.AddLease(vm.Address, vm.Hostname)
		if err != nil {
			return false, err
		}
		for _, address := range vm.SecondaryAddresses {
			err := vm.manager.DhcpServer.AddLease(address, vm.Hostname)
			if err != nil {
				vm.logger.Println(err)
			}
		}
	}
	monitorSock, err := net.Dial("unix", vm.monitorSockname)
	if err != nil {
		vm.logger.Debugf(1, "error connecting to: %s: %s\n",
			vm.monitorSockname, err)
		if err := vm.startVm(enableNetboot, haveManagerLock); err != nil {
			vm.logger.Println(err)
			vm.setState(proto.StateFailedToStart)
			return false, err
		}
		monitorSock, err = net.Dial("unix", vm.monitorSockname)
	}
	if err != nil {
		vm.logger.Println(err)
		vm.setState(proto.StateFailedToStart)
		return false, err
	}
	commandInput := make(chan string, 1)
	vm.commandInput = commandInput
	commandOutput := make(chan byte, 16<<10)
	vm.commandOutput = commandOutput
	go vm.monitor(monitorSock, commandInput, commandOutput)
	commandInput <- "qmp_capabilities"
	if vm.getDebugRoot() == "" {
		vm.setState(proto.StateRunning)
	} else {
		vm.setState(proto.StateDebugging)
	}
	if len(vm.Address.IpAddress) < 1 {
		// Must wait to see what IP address is given by external DHCP server.
		reqCh := vm.manager.DhcpServer.MakeRequestChannel(vm.Address.MacAddress)
		if dhcpTimeout < time.Minute {
			dhcpTimeout = time.Minute
		}
		timer := time.NewTimer(dhcpTimeout)
		select {
		case ipAddr := <-reqCh:
			timer.Stop()
			return false, vm.changeIpAddress(ipAddr.String())
		case <-timer.C:
			return true, errors.New("timed out on external lease")
		}
	}
	if dhcpTimeout > 0 {
		ackChan := vm.manager.DhcpServer.MakeAcknowledgmentChannel(
			vm.Address.IpAddress)
		timer := time.NewTimer(dhcpTimeout)
		select {
		case <-ackChan:
			timer.Stop()
		case <-timer.C:
			return true, nil
		}
	}
	return false, nil
}

func (vm *vmInfoType) allowMutationsAndUnlock(haveLock bool) {
	if !haveLock {
		vm.mutex.Lock()
	}
	if !vm.blockMutations {
		panic(vm.Address.IpAddress.String() +
			": blockMutations flag already unset")
	}
	vm.blockMutations = false
	vm.mutex.Unlock()
}

func (vm *vmInfoType) getBridgesAndOptions(haveManagerLock bool) (
	[]string, []string, error) {
	if !haveManagerLock {
		vm.manager.mutex.RLock()
		defer vm.manager.mutex.RUnlock()
	}
	addresses := make([]proto.Address, 1, len(vm.SecondarySubnetIDs)+1)
	addresses[0] = vm.Address
	subnetIDs := make([]string, 1, len(vm.SecondarySubnetIDs)+1)
	subnetIDs[0] = vm.SubnetId
	for index, subnetId := range vm.SecondarySubnetIDs {
		addresses = append(addresses, vm.SecondaryAddresses[index])
		subnetIDs = append(subnetIDs, subnetId)
	}
	var bridges, options []string
	deviceDriver := "virtio-net-pci"
	if vm.DisableVirtIO {
		deviceDriver = "e1000"
	}
	for index, address := range addresses {
		subnet, ok := vm.manager.subnets[subnetIDs[index]]
		if !ok {
			return nil, nil,
				fmt.Errorf("subnet: %s not found", subnetIDs[index])
		}
		bridge, vlanOption, err := vm.manager.getBridgeForSubnet(subnet)
		if err != nil {
			return nil, nil, err
		}
		bridgeIf, err := net.InterfaceByName(bridge)
		if err != nil {
			return nil, nil, err
		}
		bridges = append(bridges, bridge)
		// Shitty old systems have ancient versions of QEMU which don't support
		// the host_mtu option. So, lower the pain by only using the option if
		// MTU!=1500.
		var hostMtuOption string
		if bridgeIf.MTU != 1500 {
			hostMtuOption = fmt.Sprintf(",host_mtu=%d", bridgeIf.MTU)
		}
		options = append(options,
			"-netdev", fmt.Sprintf("tap,id=net%d,fd=%d%s",
				index, index+3, vlanOption),
			"-device", fmt.Sprintf("%s%s,netdev=net%d,mac=%s",
				deviceDriver, hostMtuOption, index, address.MacAddress))
	}
	return bridges, options, nil
}

func (vm *vmInfoType) readPid() (int, error) {
	pidfile := filepath.Join(vm.dirname, "pidfile")
	file, err := os.Open(pidfile)
	if err != nil {
		return -1, err
	}
	defer file.Close()
	var pid int
	if _, err := fmt.Fscanf(file, "%d", &pid); err != nil {
		return -1, err
	}
	return pid, nil
}

// setCpuPriority does not take any locks. It changes the priority of the
// virtualiser process.
func (vm *vmInfoType) setCpuPriority(cpuPriority int) error {
	pid, err := vm.readPid()
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("unable to read virtualiser PID, try restarting")
		}
		return fmt.Errorf("unable to read virtualiser PID: %w", err)
	}
	err = wsyscall.SetPriority(pid, cpuPriority)
	if err != nil {
		return err
	}
	return nil
}

func (vm *vmInfoType) setupLockWatcher() {
	vm.lockWatcher = lockwatcher.New(&vm.mutex,
		lockwatcher.LockWatcherOptions{
			CheckInterval: vm.manager.LockCheckInterval,
			Logger:        vm.logger,
			LogTimeout:    vm.manager.LockLogTimeout,
		})
}

func (vm *vmInfoType) startVm(enableNetboot, haveManagerLock bool) error {
	if err := checkAvailableMemory(vm.MemoryInMiB); err != nil {
		return err
	}
	nCpus := numSpecifiedVirtualCPUs(vm.MilliCPUs, vm.VirtualCPUs)
	if nCpus > uint(runtime.NumCPU()) && runtime.NumCPU() > 0 {
		nCpus = uint(runtime.NumCPU())
	}
	bridges, netOptions, err := vm.getBridgesAndOptions(haveManagerLock)
	if err != nil {
		return err
	}
	var tapFiles []*os.File
	for _, bridge := range bridges {
		tapFile, err := createTapDevice(bridge)
		if err != nil {
			return fmt.Errorf("error creating tap device: %s", err)
		}
		defer tapFile.Close()
		tapFiles = append(tapFiles, tapFile)
	}
	pidfile := filepath.Join(vm.dirname, "pidfile")
	err = vm.startQemuVm(enableNetboot, haveManagerLock, pidfile, nCpus,
		netOptions, tapFiles)
	if err != nil {
		return err
	}
	if vm.CpuPriority != 0 {
		if err := vm.setCpuPriority(vm.CpuPriority); err != nil {
			return err
		}
	}
	return nil
}

func (vm *vmInfoType) writeAndSendInfo() {
	if err := vm.writeInfo(); err != nil {
		vm.logger.Println(err)
		return
	}
	vm.manager.sendVmInfo(vm.ipAddress, &vm.VmInfo)
}

func (vm *vmInfoType) writeInfo() error {
	filename := filepath.Join(vm.dirname, "info.json")
	return json.WriteToFile(filename, fsutil.PublicFilePerms, "    ", vm)
}
