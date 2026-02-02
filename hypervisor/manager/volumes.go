package manager

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil/mounts"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver/cachingreader"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	sysClassBlock = "/sys/class/block"
)

var (
	memoryVolumeDirectory      string
	memoryVolumeDirectoryMutex sync.Mutex
)

type capacityType struct {
	available uint64 // Available to mortals.
	free      uint64 // Available to root.
	size      uint64 // Total size.
}

type mountInfo struct {
	mountEntry *mounts.MountEntry
	size       uint64
}

// calculateUsableBytes will return the number of usable bytes, based on storage
// capacity and allocation.
func calculateUsableBytes(capacity proto.StorageInfo,
	allocation uint64) uint64 {
	if allocation >= capacity.SizeBytes {
		return 0
	}
	usableBytes := capacity.SizeBytes - allocation
	if capacity.UsableBytes < usableBytes {
		usableBytes = capacity.UsableBytes
	}
	return usableBytes
}

// check2fs returns true if the device hosts an ext{2,3,4} file-system.
func check2fs(device string) bool {
	cmd := exec.Command("e2label", device)
	return cmd.Run() == nil
}

func checkTrim(mountEntry *mounts.MountEntry) bool {
	for _, option := range strings.Split(mountEntry.Options, ",") {
		if option == "discard" {
			return true
		}
	}
	return false
}

func demapDevice(device string) (string, error) {
	sysDir := filepath.Join(sysClassBlock, filepath.Base(device), "slaves")
	if file, err := os.Open(sysDir); err != nil {
		return device, nil
	} else {
		defer file.Close()
		names, err := file.Readdirnames(-1)
		if err != nil {
			return "", err
		}
		if len(names) != 1 {
			return "", fmt.Errorf("%s has %d entries", device, len(names))
		}
		return filepath.Join("/dev", names[0]), nil
	}
}

// e2fsck will check and fix an ext{2,3,4} file-system. It returns an error if
// there is a problem checking or fixing, else nil.
func e2fsck(device string) error {
	cmd := exec.Command("e2fsck", "-f", "-y", device)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if err, ok := err.(*exec.ExitError); ok {
		switch err.ExitCode() {
		case 1:
			return nil
		case 2:
			return nil
		}
	}
	output = bytes.ReplaceAll(output, carriageReturnLiteral, nil)
	output = bytes.ReplaceAll(output, newlineLiteral, newlineReplacement)
	return fmt.Errorf("error running e2fsck for: %s: %s: %s",
		device, err, string(output))
}

// e2getLabel returns the file-system label if the device hosts an ext{2,3,4}
// file-system, else it returns an error.
func e2getLabel(device string) (string, error) {
	cmd := exec.Command("e2label", device)
	if stdout, err := cmd.Output(); err != nil {
		return "", err
	} else {
		return strings.TrimSpace(string(stdout)), nil
	}
}

// e2setLabel will write a file-sytem label to an ext{2,3,4} file-system.
func e2setLabel(device, label string) error {
	return exec.Command("e2label", device, label).Run()
}

// getCapacity returns the file-system capacity for a specified directory within
// the file-system.
func getCapacity(dirname string) (capacityType, error) {
	var statbuf syscall.Statfs_t
	if err := syscall.Statfs(dirname, &statbuf); err != nil {
		return capacityType{},
			fmt.Errorf("error statfsing: %s: %s", dirname, err)
	}
	capacity := capacityType{
		available: uint64(statbuf.Bavail * uint64(statbuf.Bsize)),
		free:      uint64(statbuf.Bfree * uint64(statbuf.Bsize)),
		size:      uint64(statbuf.Blocks * uint64(statbuf.Bsize)),
	}
	return capacity, nil
}

func getMemoryVolumeDirectory(logger log.Logger) (string, error) {
	memoryVolumeDirectoryMutex.Lock()
	defer memoryVolumeDirectoryMutex.Unlock()
	if memoryVolumeDirectory != "" {
		return memoryVolumeDirectory, nil
	}
	dirname := "/tmp/hyper-volumes"
	var statbuf wsyscall.Stat_t
	if err := wsyscall.Lstat(dirname, &statbuf); err == nil {
		if statbuf.Mode&wsyscall.S_IFMT != wsyscall.S_IFDIR {
			return "", fmt.Errorf("%s is not a directory", dirname)
		}
		if statbuf.Uid != 0 {
			return "", fmt.Errorf("%s is not owned by root, UID=%d",
				dirname, statbuf.Uid)
		}
	} else if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
		return "", err
	}
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return "", err
	}
	if mountEntry := mountTable.FindEntry(dirname); mountEntry == nil {
		return "", fmt.Errorf("%s: no match in mount table", dirname)
	} else if mountEntry.Type == "tmpfs" {
		memoryVolumeDirectory = dirname
		return memoryVolumeDirectory, nil
	}
	if err := wsyscall.Mount("none", dirname, "tmpfs", 0, ""); err != nil {
		return "", err
	}
	logger.Printf("mounted tmpfs on: %s\n", dirname)
	memoryVolumeDirectory = dirname
	return memoryVolumeDirectory, nil
}

func getMounts(mountTable *mounts.MountTable) (
	map[string]*mounts.MountEntry, error) {
	mountMap := make(map[string]*mounts.MountEntry)
	for _, entry := range mountTable.Entries {
		if entry.MountPoint == "/boot" {
			continue
		}
		device := entry.Device
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		if device == "/dev/root" { // Ignore this dumb shit.
			continue
		}
		if target, err := filepath.EvalSymlinks(device); err != nil {
			return nil, err
		} else {
			device = target
		}
		var err error
		device, err = demapDevice(device)
		if err != nil {
			return nil, err
		}
		device = device[5:]
		if _, ok := mountMap[device]; !ok { // Pick the first mount point.
			mountMap[device] = entry
		}
	}
	return mountMap, nil
}

// grow2fs will try and grow an ext{2,3,4} file-system to fit the volume size,
// expanding the partition first if appropriate.
func grow2fs(volume string, logger log.DebugLogger) error {
	if check2fs(volume) {
		// Simple case: file-system is on the raw volume, no partition table.
		return resize2fs(volume, 0)
	}
	// Read MBR and check if it's a simple single-partition volume.
	file, err := os.Open(volume)
	if err != nil {
		return err
	}
	partitionTable, err := mbr.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	if partitionTable == nil {
		return fmt.Errorf("no DOS partition table found")
	}
	if partitionTable.GetPartitionSize(1) > 0 ||
		partitionTable.GetPartitionSize(2) > 0 ||
		partitionTable.GetPartitionSize(3) > 0 {
		return fmt.Errorf("unsupported partition sizes: [%s,%s,%s,%s]",
			format.FormatBytes(partitionTable.GetPartitionSize(0)),
			format.FormatBytes(partitionTable.GetPartitionSize(1)),
			format.FormatBytes(partitionTable.GetPartitionSize(2)),
			format.FormatBytes(partitionTable.GetPartitionSize(3)))
	}
	// Try and extend the partition.
	cmd := exec.Command("parted", "-s", volume, "resizepart", "1", "100%")
	if output, err := cmd.CombinedOutput(); err != nil {
		output = bytes.ReplaceAll(output, carriageReturnLiteral, nil)
		output = bytes.ReplaceAll(output, newlineLiteral, newlineReplacement)
		return fmt.Errorf("error running parted for: %s: %s: %s",
			volume, err, string(output))
	}
	// Try and resize the file-system in the partition (need a loop device).
	device, err := fsutil.LoopbackSetupAndWaitForPartition(volume, "p1",
		time.Minute, logger)
	if err != nil {
		return err
	}
	defer fsutil.LoopbackDeleteAndWaitForPartition(device, "p1", time.Minute,
		logger)
	partition := device + "p1"
	if !check2fs(partition) {
		return nil
	}
	return resize2fs(partition, 0)
}

// indexToName will return the volume name for the specified volume index (0
// is the "root" volume, 1 is "secondary-volume.0" and so on).
func indexToName(index int) string {
	if index == 0 {
		return "root"
	}
	return fmt.Sprintf("secondary-volume.%d", index-1)
}

// resize2fs will resize an ext{2,3,4} file-system to fit the specified size.
// If size is zero, it will resize to fit the device size.
func resize2fs(device string, size uint64) error {
	if err := e2fsck(device); err != nil {
		return err
	}
	cmd := exec.Command("resize2fs", device)
	if size > 0 {
		if size < 1<<20 {
			return fmt.Errorf("size: %d too small", size)
		}
		cmd.Args = append(cmd.Args, strconv.FormatUint(size>>9, 10)+"s")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		output = bytes.ReplaceAll(output, carriageReturnLiteral, nil)
		output = bytes.ReplaceAll(output, newlineLiteral, newlineReplacement)
		return fmt.Errorf("error running resize2fs for: %s: %s: %s",
			device, err, string(output))
	}
	return nil
}

// shrink2fs will try and shrink an ext{2,3,4} file-system on a volume,
// shrinking the partition afterwards if appropriate.
func shrink2fs(volume string, size uint64, logger log.DebugLogger) error {
	if check2fs(volume) {
		// Simple case: file-system is on the raw volume, no partition table.
		return resize2fs(volume, size)
	}
	// Read MBR and check if it's a simple single-partition volume.
	file, err := os.Open(volume)
	if err != nil {
		return err
	}
	partitionTable, err := mbr.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	if partitionTable == nil {
		return fmt.Errorf("no DOS partition table found")
	}
	if partitionTable.GetPartitionSize(1) > 0 ||
		partitionTable.GetPartitionSize(2) > 0 ||
		partitionTable.GetPartitionSize(3) > 0 {
		return fmt.Errorf("unsupported partition sizes: [%s,%s,%s,%s]",
			format.FormatBytes(partitionTable.GetPartitionSize(0)),
			format.FormatBytes(partitionTable.GetPartitionSize(1)),
			format.FormatBytes(partitionTable.GetPartitionSize(2)),
			format.FormatBytes(partitionTable.GetPartitionSize(3)))
	}
	size -= partitionTable.GetPartitionOffset(0)
	if size >= partitionTable.GetPartitionSize(0) {
		return errors.New("size greater than existing partition")
	}
	if err := partitionTable.SetPartitionSize(0, size); err != nil {
		return err
	}
	// Try and resize the file-system in the partition (need a loop device).
	device, err := fsutil.LoopbackSetupAndWaitForPartition(volume, "p1",
		time.Minute, logger)
	if err != nil {
		return err
	}
	deleteLoopback := true
	defer func() {
		if deleteLoopback {
			fsutil.LoopbackDeleteAndWaitForPartition(device, "p1", time.Minute,
				logger)
		}
	}()
	partition := device + "p1"
	if !check2fs(partition) {
		return errors.New("no ext2 file-system found in partition")
	}
	if err := resize2fs(partition, size); err != nil {
		return err
	}
	deleteLoopback = false
	err = fsutil.LoopbackDeleteAndWaitForPartition(device, "p1", time.Minute,
		logger)
	if err != nil {
		return err
	}
	return partitionTable.Write(volume)
}

// calculateStorageAllocations returns a table of the VM volume allocations
// (effective sizes) for each storage backend. This will grab and release the
// Manager lock.
func (m *Manager) calculateStorageAllocations() map[uint]uint64 {
	// TODO(rgooch): maintain this dynamically as VMs are mutated.
	// Collect VM infos.
	var vmInfos []proto.LocalVmInfo
	m.iterateOverVMs(0, nil, nil, nil, func(ipAddr string, vm *vmInfoType) {
		vmInfos = append(vmInfos, vm.LocalVmInfo)
	})
	allocationTable := make(map[uint]uint64) // Key: storage index.
	for _, vmInfo := range vmInfos {
		for volumeIndex, lv := range vmInfo.VolumeLocations {
			storageIndex, err := m.nameToIndex(lv.DirectoryToCleanup)
			if err != nil {
				m.Logger.Println(err)
				continue
			}
			allocationTable[storageIndex] +=
				vmInfo.Volumes[volumeIndex].EffectiveSize()
		}
	}
	return allocationTable
}

// checkFreeSpace will check if the specified backing store has sufficient
// space. It returns true if there is space. The capacities and
// allocationTable are updated if there is space.
func (m *Manager) checkFreeSpace(size uint64,
	capacities []proto.StorageInfo, allocationTable map[uint]uint64,
	storageIndex uint) bool {
	capacity := capacities[storageIndex]
	usableBytes := calculateUsableBytes(capacity, allocationTable[storageIndex])
	if size < usableBytes {
		capacity.UsableBytes -= size
		capacities[storageIndex] = capacity
		allocationTable[storageIndex] += size
		return true
	}
	return false
}

// checkFreeSpaceForVolume will check if the backing store for the specified
// volume has sufficient space. It returns nil if there is space. The
// capacityTable and allocationTable are updated if there is space.
func (m *Manager) checkFreeSpaceForVolume(volume proto.LocalVolume,
	capacities []proto.StorageInfo,
	allocationTable map[uint]uint64, size uint64) error {
	storageIndex, err := m.nameToIndex(volume.DirectoryToCleanup)
	if err != nil {
		return err
	}
	if capacities == nil {
		capacities, err = m.getCapacities()
		if err != nil {
			return err
		}
	}
	if allocationTable == nil {
		allocationTable = m.calculateStorageAllocations()
	}
	haveFreeSpace := m.checkFreeSpace(size, capacities, allocationTable,
		storageIndex)
	if !haveFreeSpace {
		return errors.New("not enough free space")
	}
	return nil
}

func (m *Manager) checkTrim(filename string) bool {
	return m.volumeInfos[filepath.Dir(filepath.Dir(filename))].CanTrim
}

func (m *Manager) detectVolumeDirectories(mountTable *mounts.MountTable) error {
	mountMap, err := getMounts(mountTable)
	if err != nil {
		return err
	}
	var mountEntriesToUse []*mounts.MountEntry
	biggestMounts := make(map[string]mountInfo)
	for device, mountEntry := range mountMap {
		sysDir := filepath.Join(sysClassBlock, device)
		linkTarget, err := os.Readlink(sysDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		_, err = os.Stat(filepath.Join(sysDir, "partition"))
		if err != nil {
			if os.IsNotExist(err) { // Not a partition: easy!
				mountEntriesToUse = append(mountEntriesToUse, mountEntry)
				continue
			}
			return err
		}
		var statbuf syscall.Statfs_t
		if err := syscall.Statfs(mountEntry.MountPoint, &statbuf); err != nil {
			return fmt.Errorf("error statfsing: %s: %s",
				mountEntry.MountPoint, err)
		}
		size := uint64(statbuf.Blocks * uint64(statbuf.Bsize))
		parentDevice := filepath.Base(filepath.Dir(linkTarget))
		if biggestMount, ok := biggestMounts[parentDevice]; !ok {
			biggestMounts[parentDevice] = mountInfo{mountEntry, size}
		} else if size > biggestMount.size {
			biggestMounts[parentDevice] = mountInfo{mountEntry, size}
		}
	}
	for _, biggestMount := range biggestMounts {
		mountEntriesToUse = append(mountEntriesToUse, biggestMount.mountEntry)
	}
	for _, entry := range mountEntriesToUse {
		volumeDirectory := filepath.Join(entry.MountPoint, "hyper-volumes")
		m.volumeDirectories = append(m.volumeDirectories, volumeDirectory)
		m.volumeInfos[volumeDirectory] = VolumeInfo{
			CanTrim:    checkTrim(entry),
			MountPoint: entry.MountPoint,
		}
	}
	sort.Strings(m.volumeDirectories)
	return nil
}

func (m *Manager) findFreeSpace(size uint64,
	capacities []proto.StorageInfo, allocationTable map[uint]uint64,
	position *uint) (
	string, error) {
	if *position >= uint(len(m.volumeDirectories)) {
		*position = 0
	}
	startingPosition := *position
	for {
		haveFreeSpace := m.checkFreeSpace(size, capacities, allocationTable,
			*position)
		if haveFreeSpace {
			dirname := m.volumeDirectories[*position]
			return dirname, nil
		}
		*position++
		if *position >= uint(len(m.volumeDirectories)) {
			*position = 0
		}
		if *position == startingPosition {
			return "", fmt.Errorf("not enough free space for %s volume",
				format.FormatBytes(size))
		}
	}
}

// getStorageCapacity returns the capacity for a specified storage index and
// accounts for the object cache and extra reserved space on the root
// file-system (if applicable) in the UsableBytes field.
func (m *Manager) getStorageCapacity(storageIndex uint) (
	proto.StorageInfo, error) {
	rawCapacity, err := getCapacity(m.volumeDirectories[storageIndex])
	if err != nil {
		return proto.StorageInfo{}, err
	}
	capacity := proto.StorageInfo{
		AvailableBytes: rawCapacity.available,
		FreeBytes:      rawCapacity.free,
		SizeBytes:      rawCapacity.size,
		UsableBytes:    rawCapacity.available,
	}
	// Remove space reserved for the object cache but not yet used.
	if m.objectCache != nil && int(storageIndex) == m.objectVolumeIndex {
		stats := m.objectCache.GetStats()
		if m.ObjectCacheBytes > stats.CachedBytes {
			unused := m.ObjectCacheBytes - stats.CachedBytes
			unused += unused >> 2 // In practice block usage is +30%.
			if unused < capacity.UsableBytes {
				capacity.UsableBytes -= unused
			} else {
				capacity.UsableBytes = 0
			}
		}
	}
	// Keep an extra 1 GiB free space for the root file-system. Be nice.
	if m.volumeInfos[m.volumeDirectories[storageIndex]].MountPoint == "/" {
		if capacity.UsableBytes > 1<<30 {
			capacity.UsableBytes -= 1 << 30
		} else {
			capacity.UsableBytes = 0
		}
	}
	return capacity, nil
}

// getCapacities returns the capacities of all storage backends.
func (m *Manager) getCapacities() ([]proto.StorageInfo, error) {
	// TODO(rgooch): abstract storage backends and reduce code duplication and
	//               add rate limiting on system calls.
	capacities := make([]proto.StorageInfo, 0, len(m.volumeDirectories))
	for storageIndex := range m.volumeDirectories {
		capacity, err := m.getStorageCapacity(uint(storageIndex))
		if err != nil {
			return nil, err
		}
		capacities = append(capacities, capacity)
	}
	return capacities, nil
}

// getVolumeDirectories will return a list of volume directories in which VM
// volumes may be created, one per specified volume. It measures the available
// storage capacity and ensures the requested volume sizes will fit.
func (m *Manager) getVolumeDirectories(rootSize uint64,
	rootVolumeType proto.VolumeType, secondaryVolumes []proto.Volume,
	spreadVolumes bool, storageIndices []uint) ([]string, error) {
	sizes := make([]uint64, 0, len(secondaryVolumes)+1)
	if rootSize > 0 {
		sizes = append(sizes, rootSize)
	}
	for _, volume := range secondaryVolumes {
		if volume.Size > 0 {
			sizes = append(sizes, volume.Size)
		} else {
			return nil, errors.New("secondary volumes cannot be zero sized")
		}
	}
	allocationTable := m.calculateStorageAllocations()
	capacities, err := m.getCapacities()
	if err != nil {
		return nil, err
	}
	directoriesToUse := make([]string, 0, len(sizes))
	var position uint
	for index, size := range sizes {
		if index < len(storageIndices) {
			haveFreeSpace := m.checkFreeSpace(size, capacities, allocationTable,
				storageIndices[index])
			if err != nil {
				return nil, err
			}
			if !haveFreeSpace {
				return nil, fmt.Errorf("storage[%d] does not have %s free",
					storageIndices[index], format.FormatBytes(size))
			}
			directoriesToUse = append(directoriesToUse,
				m.volumeDirectories[storageIndices[index]])
			continue
		}
		dirname, err := m.findFreeSpace(size, capacities, allocationTable,
			&position)
		if err != nil {
			return nil, err
		}
		directoriesToUse = append(directoriesToUse, dirname)
		if spreadVolumes {
			position++
		}
	}
	for index := range directoriesToUse {
		if (index == 0 && rootVolumeType == proto.VolumeTypeMemory) ||
			(index > 0 && index <= len(secondaryVolumes) &&
				secondaryVolumes[index-1].Type == proto.VolumeTypeMemory) {
			if dirname, err := getMemoryVolumeDirectory(m.Logger); err != nil {
				return nil, err
			} else {
				directoriesToUse[index] = dirname
			}
		}
	}
	return directoriesToUse, nil
}

// nameToIndex will return the storage index for the specified directory (which
// may be a subdirectory of one of the volumeDirectories. It returns an error
// if the name is not found.
func (m *Manager) nameToIndex(name string) (uint, error) {
	for index, volumeDirectory := range m.volumeDirectories {
		if strings.HasPrefix(name, volumeDirectory+"/") {
			return uint(index), nil
		}
	}
	return 0, fmt.Errorf("no volume directory for: %s", name)
}

func (m *Manager) setupObjectCache(mountTable *mounts.MountTable) error {
	if m.ObjectCacheBytes < 1<<20 {
		return nil
	}
	if m.ObjectCacheDirectory == "" {
		m.ObjectCacheDirectory = filepath.Join(
			filepath.Dir(m.volumeDirectories[0]),
			"objectcache")
	} else {
		m.objectVolumeIndex = -1
		mountEntry := mountTable.FindEntry(m.ObjectCacheDirectory)
		if mountEntry == nil {
			return fmt.Errorf("no mount table entry found for: %s",
				m.ObjectCacheDirectory)
		}
		for index, volumeDirectory := range m.volumeDirectories {
			if m.volumeInfos[volumeDirectory].MountPoint ==
				mountEntry.MountPoint {
				m.objectVolumeIndex = index
				break
			}
		}
	}
	if err := os.MkdirAll(m.ObjectCacheDirectory, fsutil.DirPerms); err != nil {
		return err
	}
	objSrv, err := cachingreader.NewObjectServer(m.ObjectCacheDirectory,
		m.ObjectCacheBytes, m.ImageServerAddress,
		m.Logger)
	if err != nil {
		return err
	}
	m.objectCache = objSrv
	return nil
}

func (m *Manager) setupVolumesAndObjectCache(startOptions StartOptions) error {
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return err
	}
	m.volumeInfos = make(map[string]VolumeInfo)
	if len(startOptions.VolumeDirectories) < 1 {
		if err := m.detectVolumeDirectories(mountTable); err != nil {
			return err
		}
	} else {
		m.volumeDirectories = startOptions.VolumeDirectories
		for _, dirname := range m.volumeDirectories {
			if entry := mountTable.FindEntry(dirname); entry != nil {
				m.volumeInfos[dirname] = VolumeInfo{
					CanTrim:    checkTrim(entry),
					MountPoint: entry.MountPoint,
				}
			}
		}
	}
	if len(m.volumeDirectories) < 1 {
		return errors.New("no volume directories available")
	}
	for _, volumeDirectory := range m.volumeDirectories {
		if err := os.MkdirAll(volumeDirectory, fsutil.DirPerms); err != nil {
			return err
		}
		var statbuf syscall.Statfs_t
		if err := syscall.Statfs(volumeDirectory, &statbuf); err != nil {
			return fmt.Errorf("error statfsing: %s: %s", volumeDirectory, err)
		}
		m.totalVolumeBytes += uint64(statbuf.Blocks * uint64(statbuf.Bsize))
	}
	if err := m.setupObjectCache(mountTable); err != nil {
		return err
	}
	return nil
}
