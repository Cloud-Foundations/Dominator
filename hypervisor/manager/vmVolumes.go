package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

// checkVolumes will check the volume sizes and will return an error if they
// unexpectedly changed. If grabLock is true, the VM write lock is grabbed,
// else the lock must be grabbed by the caller.
func (vm *vmInfoType) checkVolumes(grabLock bool) error {
	if grabLock {
		vm.mutex.Lock()
		defer vm.mutex.Unlock()
	}
	for index, volume := range vm.VolumeLocations {
		expectedSize := vm.Volumes[index].Size
		if fi, err := os.Stat(volume.Filename); err != nil {
			return fmt.Errorf("error stating volume[%d]: %s", index, err)
		} else if foundSize := uint64(fi.Size()); foundSize != expectedSize {
			if vm.Volumes[index].Format == proto.VolumeFormatQCOW2 {
				vm.Volumes[index].Size = foundSize
				continue
			}
			return fmt.Errorf("volume[%d] size expected: %s, found: %s",
				index, format.FormatBytes(expectedSize),
				format.FormatBytes(uint64(fi.Size())))
		}
	}
	return nil
}

func (vm *vmInfoType) scanSnapshots() error {
	// Build a map of all filenames in VM volume directories.
	dirnameToFilenames := make(map[string]map[string]struct{})
	for _, volumeLocation := range vm.VolumeLocations {
		dirnameToFilenames[volumeLocation.DirectoryToCleanup] = nil
	}
	for dirname := range dirnameToFilenames {
		filenames, err := fsutil.ReadDirnames(dirname, false)
		if err != nil {
			return err
		}
		dirnameToFilenames[dirname] = stringutil.ConvertListToMap(filenames,
			false)
	}
	// Look for snapshots and build new []proto.Volume.
	newVolumes := make([]proto.Volume, len(vm.Volumes))
	for index, vl := range vm.VolumeLocations {
		newVolumes[index] = vm.Volumes[index]
		snapshots := make(map[string]uint64)
		newVolumes[index].Snapshots = snapshots
		volumeName := indexToName(index)
		snapshotBase := volumeName + ".snapshot"
		for filename := range dirnameToFilenames[vl.DirectoryToCleanup] {
			pathname := filepath.Join(vl.DirectoryToCleanup, filename)
			fi, err := os.Stat(pathname)
			if err != nil {
				return fmt.Errorf("cannot stat: %s: %s\n", pathname, err)
			}
			if strings.HasPrefix(filename, snapshotBase) {
				suffix := filename[len(snapshotBase):]
				if suffix == "" {
					snapshots[suffix] = uint64(fi.Size())

				} else if suffix[0] == ':' {
					snapshots[suffix[1:]] = uint64(fi.Size())
				}
			}
		}
	}
	// Compare with old volumes.
	var changed bool
	for index, oldVolume := range vm.Volumes {
		if !oldVolume.Equal(&newVolumes[index]) {
			changed = true
			break
		}
	}
	if changed {
		vm.logger.Printf("scanSnapshots(): snapshots changed: %v -> %v\n",
			vm.Volumes, newVolumes)
		vm.Volumes = newVolumes
		if err := vm.writeInfo(); err != nil {
			return err
		}
	}
	return nil
}

func (vm *vmInfoType) setupVolumes(rootSize uint64,
	rootVolumeType proto.VolumeType, secondaryVolumes []proto.Volume,
	spreadVolumes bool, storageIndices []uint) error {
	volumeDirectories, err := vm.manager.getVolumeDirectories(rootSize,
		rootVolumeType, secondaryVolumes, spreadVolumes, storageIndices)
	if err != nil {
		return err
	}
	volumeDirectory := filepath.Join(volumeDirectories[0], vm.ipAddress)
	os.RemoveAll(volumeDirectory)
	if err := os.MkdirAll(volumeDirectory, fsutil.DirPerms); err != nil {
		return err
	}
	filename := filepath.Join(volumeDirectory, "root")
	vm.VolumeLocations = append(vm.VolumeLocations,
		proto.LocalVolume{volumeDirectory, filename})
	for index := range secondaryVolumes {
		volumeDirectory := filepath.Join(volumeDirectories[index+1],
			vm.ipAddress)
		os.RemoveAll(volumeDirectory)
		if err := os.MkdirAll(volumeDirectory, fsutil.DirPerms); err != nil {
			return err
		}
		filename := filepath.Join(volumeDirectory, indexToName(index+1))
		vm.VolumeLocations = append(vm.VolumeLocations,
			proto.LocalVolume{volumeDirectory, filename})
	}
	return nil
}
