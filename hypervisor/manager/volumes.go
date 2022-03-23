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
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil/mounts"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	sysClassBlock = "/sys/class/block"
)

type mountInfo struct {
	mountEntry *mounts.MountEntry
	size       uint64
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

func getFreeSpace(dirname string, freeSpaceTable map[string]uint64) (
	uint64, error) {
	if freeSpace, ok := freeSpaceTable[dirname]; ok {
		return freeSpace, nil
	}
	var statbuf syscall.Statfs_t
	if err := syscall.Statfs(dirname, &statbuf); err != nil {
		return 0, fmt.Errorf("error statfsing: %s: %s", dirname, err)
	}
	freeSpace := uint64(statbuf.Bfree * uint64(statbuf.Bsize))
	freeSpaceTable[dirname] = freeSpace
	return freeSpace, nil
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
// expanding the partition if appropriate.
func grow2fs(volume string) error {
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
	device, err := fsutil.LoopbackSetup(volume)
	if err != nil {
		return err
	}
	defer fsutil.LoopbackDelete(device)
	partition := device + "p1"
	if !check2fs(partition) {
		return nil
	}
	return resize2fs(partition, 0)
}

// resize2fs will resize an ext{2,3,4} file-system to fit the specified size.
// If size is zero, it will resize to fit the device size.
func resize2fs(device string, size uint64) error {
	cmd := exec.Command("e2fsck", "-f", "-y", device)
	if output, err := cmd.CombinedOutput(); err != nil {
		output = bytes.ReplaceAll(output, carriageReturnLiteral, nil)
		output = bytes.ReplaceAll(output, newlineLiteral, newlineReplacement)
		return fmt.Errorf("error running e2fsck for: %s: %s: %s",
			device, err, string(output))
	}
	cmd = exec.Command("resize2fs", device)
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

// shrink2fs will try and shrink an ext{2,3,4} file-system on a volume.
func shrink2fs(volume string, size uint64) error {
	if check2fs(volume) {
		// Simple case: file-system is on the raw volume, no partition table.
		return resize2fs(volume, size)
	}
	return errors.New("partition shrinking not supported")
}

func (m *Manager) checkTrim(filename string) bool {
	return m.volumeInfos[filepath.Dir(filepath.Dir(filename))].canTrim
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
		m.volumeInfos[volumeDirectory] = volumeInfo{canTrim: checkTrim(entry)}
	}
	sort.Strings(m.volumeDirectories)
	return nil
}

func (m *Manager) findFreeSpace(size uint64, freeSpaceTable map[string]uint64,
	position *int) (string, error) {
	if *position >= len(m.volumeDirectories) {
		*position = 0
	}
	startingPosition := *position
	for {
		freeSpace, err := getFreeSpace(m.volumeDirectories[*position],
			freeSpaceTable)
		if err != nil {
			return "", err
		}
		if size < freeSpace {
			dirname := m.volumeDirectories[*position]
			freeSpaceTable[dirname] -= size
			return dirname, nil
		}
		*position++
		if *position >= len(m.volumeDirectories) {
			*position = 0
		}
		if *position == startingPosition {
			return "", fmt.Errorf("not enough free space for %s volume",
				format.FormatBytes(size))
		}
	}
}

func (m *Manager) getVolumeDirectories(rootSize uint64,
	volumes []proto.Volume, spreadVolumes bool) ([]string, error) {
	sizes := make([]uint64, 0, len(volumes)+1)
	if rootSize > 0 {
		sizes = append(sizes, rootSize)
	}
	for _, volume := range volumes {
		if volume.Size > 0 {
			sizes = append(sizes, volume.Size)
		} else {
			return nil, errors.New("secondary volumes cannot be zero sized")
		}
	}
	freeSpaceTable := make(map[string]uint64, len(m.volumeDirectories))
	directoriesToUse := make([]string, 0, len(sizes))
	position := 0
	for len(sizes) > 0 {
		dirname, err := m.findFreeSpace(sizes[0], freeSpaceTable, &position)
		if err != nil {
			return nil, err
		}
		directoriesToUse = append(directoriesToUse, dirname)
		sizes = sizes[1:]
		if spreadVolumes {
			position++
		}
	}
	return directoriesToUse, nil
}

func (m *Manager) setupVolumes(startOptions StartOptions) error {
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return err
	}
	m.volumeInfos = make(map[string]volumeInfo)
	if len(startOptions.VolumeDirectories) < 1 {
		if err := m.detectVolumeDirectories(mountTable); err != nil {
			return err
		}
	} else {
		m.volumeDirectories = startOptions.VolumeDirectories
		for _, dirname := range m.volumeDirectories {
			if entry := mountTable.FindEntry(dirname); entry != nil {
				m.volumeInfos[dirname] = volumeInfo{canTrim: checkTrim(entry)}
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
	return nil
}
