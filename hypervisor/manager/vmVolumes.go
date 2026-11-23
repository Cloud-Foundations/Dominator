package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/images/qcow2"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
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

// listVolumePathnames will list all the possible pathnames for the specified
// volume. If checkPresent is true, only pathnames which are present will be
// returned.
func (vm *vmInfoType) listVolumePathnames(volumeIndex uint, checkPresent bool) (
	[]string, error) {
	possiblePathnames := []string{vm.VolumeLocations[volumeIndex].Filename}
	if volumeIndex < 1 {
		possiblePathnames = append(possiblePathnames,
			vm.getInitrdPath(),
			vm.getKernelPath(),
		)
	}
	if !checkPresent {
		return possiblePathnames, nil
	}
	pathnames := make([]string, 0, len(possiblePathnames))
	for _, pathname := range possiblePathnames {
		if _, err := os.Stat(pathname); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		pathnames = append(pathnames, pathname)
	}
	return pathnames, nil
}

func (vm *vmInfoType) scanStorage() error {
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
	// Look for QCOW2 root volumes and snapshots and build new []proto.Volume.
	newVolumes := make([]proto.Volume, 0, len(vm.Volumes))
	for index, vl := range vm.VolumeLocations {
		volume := vm.Volumes[index]
		// Read QCOW2 header for root volumes. This should be cheap enough.
		if index == 0 && volume.Format == proto.VolumeFormatQCOW2 {
			header, err := qcow2.ReadHeaderFromFile(vl.Filename)
			if err != nil {
				return err
			}
			volume.VirtualSize = header.Size
		}
		snapshots := make(map[string]uint64)
		volume.Snapshots = snapshots
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
		newVolumes = append(newVolumes, volume)
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
		vm.logger.Printf("scanStorage(): storage changed: %v -> %v\n",
			vm.Volumes, newVolumes)
		vm.Volumes = newVolumes
		if err := vm.writeInfo(); err != nil {
			return err
		}
	}
	return nil
}

// setupVolumes will allocate space for the VM volumes. It measures the
// available storage capacity and ensures the requested volume sizes will fit.
// This will grab and release the Manager read lock and each VM read lock, and
// later the VM write lock.
// Finally, an update message is broadcast. This gives early visibility into VM
// creation, before volumes are populated.
func (vm *vmInfoType) setupVolumes(rootVolume proto.Volume,
	secondaryVolumes []proto.Volume, spreadVolumes bool,
	storageIndices []uint) error {
	switch rootVolume.Interface {
	case proto.VolumeInterfaceDFM:
		return fmt.Errorf("root volume not supported on interface: %s",
			rootVolume.Interface)
	}
	volumeDirectories, err := vm.manager.getVolumeDirectories(rootVolume,
		secondaryVolumes, spreadVolumes, storageIndices)
	if err != nil {
		return err
	}
	volumeDirectory := filepath.Join(volumeDirectories[0], vm.ipAddress)
	os.RemoveAll(volumeDirectory)
	if err := os.MkdirAll(volumeDirectory, fsutil.DirPerms); err != nil {
		return err
	}
	filename := filepath.Join(volumeDirectory, "root")
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	vm.Volumes = append(vm.Volumes, rootVolume)
	vm.VolumeLocations = append(vm.VolumeLocations,
		proto.LocalVolume{volumeDirectory, filename})
	for index, volume := range secondaryVolumes {
		volumeDirectory := filepath.Join(volumeDirectories[index+1],
			vm.ipAddress)
		os.RemoveAll(volumeDirectory)
		if err := os.MkdirAll(volumeDirectory, fsutil.DirPerms); err != nil {
			return err
		}
		filename := filepath.Join(volumeDirectory, indexToName(index+1))
		vm.Volumes = append(vm.Volumes, volume)
		vm.VolumeLocations = append(vm.VolumeLocations,
			proto.LocalVolume{volumeDirectory, filename})
	}
	vm.manager.sendVmInfo(vm.ipAddress, &vm.VmInfo)
	return nil
}

func (vm *vmInfoType) writeQcow2RootVolume(conn *srpc.Conn, client srpc.ClientI,
	img *imageType, writeRawOptions util.WriteRawOptions,
	sendUpdate func(conn *srpc.Conn, message string) error) error {
	if len(writeRawOptions.OverlayDirectories) > 0 {
		return fmt.Errorf("overlay directories not supported with QCOW2")
	}
	if len(writeRawOptions.OverlayFiles) > 0 {
		return fmt.Errorf("overlay files not supported with QCOW2")
	}
	imageName := writeRawOptions.InitialImageName
	volumeDir := vm.VolumeLocations[0].DirectoryToCleanup
	cacheParentDir := filepath.Dir(volumeDir)
	_, err := vm.manager.cacheVolumeInDir(cacheParentDir, imageName,
		conn, client, img, writeRawOptions.MinimumFreeBytes,
		writeRawOptions.RoundupPower, sendUpdate, vm.logger)
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(cacheParentDir, "cache", imageName)
	rootPathname := filepath.Join(cacheDir, "root")
	backingPathname := filepath.Join(volumeDir, "root.image")
	if err := os.Link(rootPathname, backingPathname); err != nil {
		return err
	}
	// Check size in case there was a purge followed by a create before the link
	fi, err := os.Stat(backingPathname)
	if err != nil {
		return fmt.Errorf("error stating RAW file: %s: %s",
			backingPathname, err)
	}
	if uint64(fi.Size()) < vm.Volumes[0].Size {
		os.Remove(backingPathname)
		return fmt.Errorf("RAW file: %s too small, need: %s, have: %s",
			backingPathname, vm.Volumes[0].Size, fi.Size())
	}
	os.Link(filepath.Join(cacheDir, "initrd"),
		filepath.Join(volumeDir, "initrd"))
	os.Link(filepath.Join(cacheDir, "kernel"),
		filepath.Join(volumeDir, "kernel"))
	cmd := exec.Command("qemu-img", "create",
		"-f", "qcow2",
		"-b", "root.image",
		"-F", "raw",
		vm.VolumeLocations[0].Filename)
	cmd.Dir = volumeDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running %v: %s: %s",
			cmd.Args, err, string(output))
	}
	err = os.Chmod(vm.VolumeLocations[0].Filename, fsutil.PrivateFilePerms)
	if err != nil {
		return err
	}
	vm.RootFileSystemLabel = "rootfs@image"
	return nil
}

func (vm *vmInfoType) writeRawRootVolume(conn *srpc.Conn, client srpc.ClientI,
	firmwareType proto.FirmwareType, img *imageType,
	skipBootloader bool, writeRawOptions util.WriteRawOptions,
	sendUpdate func(conn *srpc.Conn, message string) error) error {
	imageName := writeRawOptions.InitialImageName
	if err := sendUpdate(conn, "unpacking image: "+imageName); err != nil {
		return err
	}
	vm.logger.Printf("Unpacking image: %s\n", imageName)
	fs := img.image.FileSystem
	err := vm.manager.writeRaw(vm.VolumeLocations[0], "", client, fs,
		firmwareType, writeRawOptions, skipBootloader, vm.logger)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(vm.VolumeLocations[0].Filename); err != nil {
		return err
	} else if vm.Volumes[0].Size != uint64(fi.Size()) {
		vm.logger.Printf("Changing root volume size from: %s to: %s\n",
			format.FormatBytes(vm.Volumes[0].Size),
			format.FormatBytes(uint64(fi.Size())))
		vm.mutex.Lock()
		vm.Volumes[0].Size = uint64(fi.Size())
		vm.mutex.Unlock()
	}
	return nil
}

func (vm *vmInfoType) writeRootVolume(conn *srpc.Conn, client srpc.ClientI,
	firmwareType proto.FirmwareType, img *imageType,
	skipBootloader bool, writeRawOptions util.WriteRawOptions,
	sendUpdate func(conn *srpc.Conn, message string) error) error {
	switch vm.Volumes[0].Format {
	case proto.VolumeFormatRaw:
		return vm.writeRawRootVolume(conn, client, firmwareType, img,
			skipBootloader, writeRawOptions, sendUpdate)
	case proto.VolumeFormatQCOW2:
		return vm.writeQcow2RootVolume(conn, client, img, writeRawOptions,
			sendUpdate)
	default:
		return fmt.Errorf("unsupported root volume format: %s",
			vm.Volumes[0].Format)
	}
}
