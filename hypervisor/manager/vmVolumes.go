package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (vm *vmInfoType) checkVolumes(grabLock bool) error {
	if grabLock {
		vm.mutex.RLock()
		defer vm.mutex.RUnlock()
	}
	for index, volume := range vm.VolumeLocations {
		expectedSize := vm.Volumes[index].Size
		if fi, err := os.Stat(volume.Filename); err != nil {
			return fmt.Errorf("error stating volume[%d]: %s", index, err)
		} else if fi.Size() != int64(expectedSize) {
			return fmt.Errorf("volume[%d] size expected: %s, found: %s",
				index, format.FormatBytes(expectedSize),
				format.FormatBytes(uint64(fi.Size())))
		}
	}
	return nil
}

func (vm *vmInfoType) setupVolumes(rootSize uint64,
	rootVolumeType proto.VolumeType, secondaryVolumes []proto.Volume,
	spreadVolumes bool) error {
	volumeDirectories, err := vm.manager.getVolumeDirectories(rootSize,
		secondaryVolumes, spreadVolumes)
	if err != nil {
		return err
	}
	for index := range volumeDirectories {
		if (index == 0 && rootVolumeType == proto.VolumeTypeMemory) ||
			(index > 0 && index <= len(secondaryVolumes) &&
				secondaryVolumes[index-1].Type == proto.VolumeTypeMemory) {
			if dirname, err := getMemoryVolumeDirectory(vm.logger); err != nil {
				return err
			} else {
				volumeDirectories[index] = dirname
			}
		}
	}
	volumeDirectory := filepath.Join(volumeDirectories[0], vm.ipAddress)
	os.RemoveAll(volumeDirectory)
	if err := os.MkdirAll(volumeDirectory, dirPerms); err != nil {
		return err
	}
	filename := filepath.Join(volumeDirectory, "root")
	vm.VolumeLocations = append(vm.VolumeLocations,
		proto.LocalVolume{volumeDirectory, filename})
	for index := range secondaryVolumes {
		volumeDirectory := filepath.Join(volumeDirectories[index+1],
			vm.ipAddress)
		os.RemoveAll(volumeDirectory)
		if err := os.MkdirAll(volumeDirectory, dirPerms); err != nil {
			return err
		}
		filename := filepath.Join(volumeDirectory, indexToName(index+1))
		vm.VolumeLocations = append(vm.VolumeLocations,
			proto.LocalVolume{volumeDirectory, filename})
	}
	return nil
}
