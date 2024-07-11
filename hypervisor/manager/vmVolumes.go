package manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
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

func (vm *vmInfoType) setupVolumes(rootSize uint64,
	rootVolumeType proto.VolumeType, secondaryVolumes []proto.Volume,
	spreadVolumes bool) error {
	volumeDirectories, err := vm.manager.getVolumeDirectories(rootSize,
		rootVolumeType, secondaryVolumes, spreadVolumes)
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
