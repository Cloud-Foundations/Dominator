//go:build linux
// +build linux

package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	imageclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/cpusharer"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	installer_proto "github.com/Cloud-Foundations/Dominator/proto/installer"
)

const (
	bootMountPoint = "/boot"
	efiMountPoint  = "/mnt/efi"
	rootMountPoint = "/"

	bootFsLabel = "bootfs"
	efiFsLabel  = "EFI"
	rootFsLabel = "rootfs"

	keyFile        = "/etc/crypt.key"
	sysFirmwareEfi = "/sys/firmware/efi"
)

type driveType struct {
	busLocation string
	discarded   bool
	devpath     string
	mbr         *mbr.Mbr
	name        string
	size        uint64 // Bytes
}

type kexecRebooter struct {
	logger log.DebugLogger
}

func init() {
	gob.Register(&filesystem.RegularInode{})
	gob.Register(&filesystem.SymlinkInode{})
	gob.Register(&filesystem.SpecialInode{})
	gob.Register(&filesystem.DirectoryInode{})
}

// Returns true if the system has EFI firmware, else false.
func checkIsEfi() bool {
	if _, err := os.Stat(sysFirmwareEfi); err == nil {
		return true
	}
	return false
}

func closeEncryptedVolumes(logger log.DebugLogger) error {
	if file, err := os.Open("/dev/mapper"); err != nil {
		return err
	} else {
		defer file.Close()
		if names, err := file.Readdirnames(-1); err != nil {
			return err
		} else {
			for _, name := range names {
				if name == "control" {
					continue
				}
				err := run("cryptsetup", *tmpRoot, logger, "close", name)
				if err != nil {
					return err
				}
			}
			return nil
		}
	}
}

func configureBootDrive(cpuSharer cpusharer.CpuSharer, drive *driveType,
	layout installer_proto.StorageLayout, rootPartition, bootPartition int,
	img *image.Image, objGetter objectserver.ObjectsGetter,
	bootInfo *util.BootInfoType, logger log.DebugLogger) error {
	startTime := time.Now()
	if run("blkdiscard", "", logger, drive.devpath) == nil {
		drive.discarded = true
		logger.Printf("discarded %s in %s\n",
			drive.devpath, format.Duration(time.Since(startTime)))
	} else { // Erase old partition.
		if err := eraseStart(drive.devpath, logger); err != nil {
			return err
		}
	}
	isEfi := checkIsEfi()
	args := []string{"-s", "-a", "optimal", drive.devpath}
	if isEfi {
		args = append(args, "mklabel", "gpt")
	} else {
		args = append(args, "mklabel", "msdos")
	}
	unitSize := uint64(1 << 20)
	unitSuffix := "MiB"
	offsetInUnits := uint64(1)
	for _, partition := range layout.BootDriveLayout {
		sizeInUnits := partition.MinimumFreeBytes / unitSize
		if sizeInUnits*unitSize < partition.MinimumFreeBytes {
			sizeInUnits++
		}
		var partType string
		switch partition.FileSystemType {
		case installer_proto.FileSystemTypeVfat:
			partType = "fat32"
		default:
			partType = partition.FileSystemType.String()
		}
		args = append(args, "mkpart", "primary", partType,
			strconv.FormatUint(offsetInUnits, 10)+unitSuffix,
			strconv.FormatUint(offsetInUnits+sizeInUnits, 10)+unitSuffix)
		offsetInUnits += sizeInUnits
	}
	args = append(args, "mkpart", "primary", "ext2",
		strconv.FormatUint(offsetInUnits, 10)+unitSuffix, "100%")
	if isEfi { // EFI System Partition is always the first partition.
		args = append(args, "set", "1", "esp", "on")
	} else {
		args = append(args,
			"set", strconv.FormatInt(int64(bootPartition), 10), "boot", "on")
	}
	if err := run("parted", *tmpRoot, logger, args...); err != nil {
		return err
	}
	// Prepare all file-systems concurrently, make them serially.
	concurrentState := concurrent.NewState(uint(
		len(layout.BootDriveLayout) + 1))
	var mkfsMutex sync.Mutex
	for index, partition := range layout.BootDriveLayout {
		device := partitionName(drive.devpath, index+1)
		partition := partition
		err := concurrentState.GoRun(func() error {
			return drive.makeFileSystem(cpuSharer, device, partition.MountPoint,
				partition.FileSystemType, layout.Encrypt, &mkfsMutex, 0, logger)
		})
		if err != nil {
			return err
		}
	}
	concurrentState.GoRun(func() error {
		device := partitionName(drive.devpath, len(layout.BootDriveLayout)+1)
		return drive.makeFileSystem(cpuSharer, device,
			layout.ExtraMountPointsBasename+"0",
			installer_proto.FileSystemTypeExt4, layout.Encrypt, &mkfsMutex,
			65536, logger)
	})
	if err := concurrentState.Reap(); err != nil {
		return err
	}
	// Mount all file-systems, except the /boot and data file-systems, so that
	// the image can create directories in them. First do the root partition,
	// which might not be first in the list.
	err := mount(partitionName(drive.devpath, rootPartition), *mountPoint,
		layout.BootDriveLayout[rootPartition-1].FileSystemType.String(), logger)
	if err != nil {
		return err
	}
	for index, partition := range layout.BootDriveLayout {
		if index+1 == rootPartition || index+1 == bootPartition {
			continue
		}
		device := partitionName(drive.devpath, index+1)
		err := mount(remapDevice(device, partition.MountPoint, layout.Encrypt),
			filepath.Join(*mountPoint, partition.MountPoint),
			partition.FileSystemType.String(), logger)
		if err != nil {
			return err
		}
	}
	var bootP int
	if bootPartition != rootPartition {
		bootP = bootPartition
	}
	return installRoot(drive.devpath, layout, img.FileSystem, objGetter,
		bootInfo, bootP, logger)
}

func configureDataDrive(cpuSharer cpusharer.CpuSharer, drive *driveType,
	index int, layout installer_proto.StorageLayout,
	logger log.DebugLogger) error {
	startTime := time.Now()
	if run("blkdiscard", "", logger, drive.devpath) == nil {
		drive.discarded = true
		logger.Printf("discarded %s in %s\n",
			drive.devpath, format.Duration(time.Since(startTime)))
	}
	dataMountPoint := layout.ExtraMountPointsBasename + strconv.FormatInt(
		int64(index), 10)
	return drive.makeFileSystem(cpuSharer, drive.devpath, dataMountPoint,
		installer_proto.FileSystemTypeExt4, layout.Encrypt, nil, 1048576,
		logger)
}

func configureStorage(config fm_proto.GetMachineInfoResponse,
	logger log.DebugLogger) (Rebooter, error) {
	startTime := time.Now()
	var layout installer_proto.StorageLayout
	err := json.ReadFromFile(filepath.Join(*tftpDirectory,
		"storage-layout.json"),
		&layout)
	if err != nil {
		return nil, err
	}
	isEfi := checkIsEfi()
	if isEfi {
		newPartitions := make([]installer_proto.Partition, 1,
			len(layout.BootDriveLayout)+1)
		newPartitions[0] = installer_proto.Partition{
			FileSystemType:   installer_proto.FileSystemTypeVfat,
			MountPoint:       efiMountPoint,
			MinimumFreeBytes: 128 << 20,
		}
		for _, partition := range layout.BootDriveLayout {
			if partition.MountPoint == bootMountPoint {
				newPartitions[0].MountPoint = bootMountPoint
				if partition.MinimumFreeBytes >
					newPartitions[0].MinimumFreeBytes {
					newPartitions[0].MinimumFreeBytes =
						partition.MinimumFreeBytes
				}
				continue
			}
			newPartitions = append(newPartitions, partition)
		}
		layout.BootDriveLayout = newPartitions
	}
	var bootPartition, rootPartition int
	for index, partition := range layout.BootDriveLayout {
		switch partition.MountPoint {
		case rootMountPoint:
			rootPartition = index + 1
		case bootMountPoint:
			bootPartition = index + 1
		}
	}
	if rootPartition < 1 {
		return nil, fmt.Errorf("no root partition specified in layout")
	}
	if bootPartition < 1 {
		bootPartition = rootPartition
	}
	drives, err := listDrives(logger)
	if err != nil {
		return nil, err
	}
	drives, err = selectDrives(drives, logger)
	if err != nil {
		return nil, err
	}
	rootDevice := partitionName(drives[0].devpath, rootPartition)
	var randomKey []byte
	if layout.Encrypt {
		randomKey, err = getRandomKey(16, logger)
		if err != nil {
			return nil, err
		}
	}
	imageName, err := readString(filepath.Join(*tftpDirectory, "imagename"),
		true)
	if err != nil {
		return nil, err
	}
	imageName, img, client, err := getImage(imageName, logger)
	if err != nil {
		return nil, err
	}
	if client != nil {
		defer client.Close()
	}
	if img == nil {
		logger.Println("no image specified, skipping paritioning")
		return nil, nil
	}
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return nil, err
	}
	imageSize := img.FileSystem.EstimateUsage(0)
	if layout.BootDriveLayout[rootPartition-1].MinimumFreeBytes <
		imageSize {
		layout.BootDriveLayout[rootPartition-1].MinimumFreeBytes = imageSize
	}
	layout.BootDriveLayout[rootPartition-1].MinimumFreeBytes += imageSize
	bootInfo, err := util.GetBootInfo(img.FileSystem, rootFsLabel, "")
	if err != nil {
		return nil, err
	}
	var rebooter Rebooter
	if layout.UseKexec {
		rebooter = kexecRebooter{logger: logger}
	}
	objClient := objectclient.AttachObjectClient(client)
	defer objClient.Close()
	objGetter, err := createObjectsCache(img.FileSystem.GetObjects(), objClient,
		rootDevice, logger)
	if err != nil {
		return nil, err
	}
	toolsFileSystem := img.FileSystem
	toolsImageName, err := readString(filepath.Join(*tftpDirectory,
		"tools-imagename"), true)
	if err != nil {
		return nil, err
	}
	if toolsImageName != "" {
		_, img, err := getImageFromClient(client, toolsImageName, true, logger)
		if err != nil {
			return nil, err
		}
		if img != nil {
			if err := img.FileSystem.RebuildInodePointers(); err != nil {
				return nil, err
			}
			err = objGetter.downloadMissing(img.FileSystem.GetObjects(),
				objClient, logger)
			if err != nil {
				return nil, err
			}
			toolsFileSystem = img.FileSystem
		}
	}
	if err := installTmpRoot(toolsFileSystem, objGetter, logger); err != nil {
		return nil, err
	}
	if len(randomKey) > 0 {
		err = ioutil.WriteFile(filepath.Join(*tmpRoot, keyFile), randomKey,
			fsutil.PrivateFilePerms)
		if err != nil {
			return nil, err
		}
		for index := range randomKey { // Scrub key.
			randomKey[index] = 0
		}
	}
	// Configure all drives concurrently, making file-systems.
	// Use concurrent package because of it's reaping cabability.
	// Use cpusharer package to limit CPU intensive operations.
	concurrentState := concurrent.NewState(uint(len(drives)))
	cpuSharer := cpusharer.NewFifoCpuSharer()
	err = concurrentState.GoRun(func() error {
		return configureBootDrive(cpuSharer, drives[0], layout, rootPartition,
			bootPartition, img, objGetter, bootInfo, logger)
	})
	if err != nil {
		return nil, concurrentState.Reap()
	}
	for index, drive := range drives[1:] {
		drive := drive
		index := index + 1
		err := concurrentState.GoRun(func() error {
			return configureDataDrive(cpuSharer, drive, index, layout, logger)
		})
		if err != nil {
			break
		}
	}
	if err := concurrentState.Reap(); err != nil {
		return nil, err
	}
	// Make table entries for the boot device file-systems, except data FS.
	fsTab := &bytes.Buffer{}
	cryptTab := &bytes.Buffer{}
	// Write the root file-system entry first.
	bootCheckCount := uint(1)
	{
		device := partitionName(drives[0].devpath, rootPartition)
		partition := layout.BootDriveLayout[rootPartition-1]
		err = drives[0].writeDeviceEntries(device, partition.MountPoint,
			partition.FileSystemType, fsTab, cryptTab, bootCheckCount)
		if err != nil {
			return nil, err
		}
	}
	for index, partition := range layout.BootDriveLayout {
		if index+1 == rootPartition {
			continue
		}
		bootCheckCount++
		device := partitionName(drives[0].devpath, index+1)
		err = drives[0].writeDeviceEntries(device, partition.MountPoint,
			partition.FileSystemType, fsTab, cryptTab, bootCheckCount)
		if err != nil {
			return nil, err
		}
	}
	// Make table entries for data file-systems.
	for index, drive := range drives {
		checkCount := uint(2)
		var device string
		if index == 0 { // The boot device is partitioned.
			checkCount = uint(len(layout.BootDriveLayout) + 1)
			device = partitionName(drives[0].devpath,
				len(layout.BootDriveLayout)+1)
		} else { // Extra drives are used whole.
			device = drive.devpath
		}
		dataMountPoint := layout.ExtraMountPointsBasename + strconv.FormatInt(
			int64(index), 10)
		err = drive.writeDeviceEntries(device, dataMountPoint,
			installer_proto.FileSystemTypeExt4, fsTab, cryptTab, checkCount)
		if err != nil {
			return nil, err
		}
	}
	logger.Printf("Writing /etc/fstab:\n%s", string(fsTab.Bytes()))
	err = ioutil.WriteFile(filepath.Join(*mountPoint, "etc", "fstab"),
		fsTab.Bytes(), fsutil.PublicFilePerms)
	if err != nil {
		return nil, err
	}
	if len(randomKey) > 0 {
		logger.Printf("Writing /etc/crypttab:\n%s", string(cryptTab.Bytes()))
		err = ioutil.WriteFile(filepath.Join(*mountPoint, "/etc", "crypttab"),
			cryptTab.Bytes(), fsutil.PublicFilePerms)
		if err != nil {
			return nil, err
		}
		// Copy key file and scrub temporary copy.
		tmpKeyFile := filepath.Join(*tmpRoot, keyFile)
		err = fsutil.CopyFile(filepath.Join(*mountPoint, keyFile),
			tmpKeyFile, fsutil.PrivateFilePerms)
		if err != nil {
			return nil, err
		}
		if file, err := os.OpenFile(tmpKeyFile, os.O_WRONLY, 0); err != nil {
			return nil, err
		} else {
			defer file.Close()
			if _, err := file.Write(randomKey); err != nil {
				return nil, err
			}
		}
	}
	// Copy configuration and log data.
	logdir := filepath.Join(*mountPoint, "var", "log", "installer")
	if err := os.MkdirAll(logdir, fsutil.DirPerms); err != nil {
		return nil, err
	}
	if err := fsutil.CopyTree(logdir, *tftpDirectory); err != nil {
		return nil, err
	}
	dhcpLogdir := "/var/log/installer/dhcp"
	if _, err := os.Stat(dhcpLogdir); err == nil {
		destdir := filepath.Join(*mountPoint, dhcpLogdir)
		if err := os.Mkdir(destdir, fsutil.DirPerms); err != nil {
			return nil, err
		}
		if err := fsutil.CopyTree(destdir, dhcpLogdir); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(etcFilename); err == nil {
		destfile := filepath.Join(*mountPoint, etcFilename)
		err := fsutil.CopyFile(destfile, etcFilename, fsutil.PublicFilePerms)
		if err != nil {
			return nil, err
		}
	}
	if err := util.WriteImageName(*mountPoint, imageName); err != nil {
		return nil, err
	}
	logger.Printf("configureStorage() took %s\n",
		format.Duration(time.Since(startTime)))
	return rebooter, nil
}

func eraseStart(device string, logger log.DebugLogger) error {
	if *dryRun {
		logger.Debugf(0, "dry run: skipping erasure of: %s\n", device)
		return nil
	}
	logger.Debugf(0, "erasing start of: %s\n", device)
	file, err := os.OpenFile(device, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	var buffer [65536]byte
	if _, err := file.Write(buffer[:]); err != nil {
		return err
	}
	return nil
}

func getImageserverAddress() (string, error) {
	if *imageServerHostname != "" {
		return fmt.Sprintf("%s:%d", *imageServerHostname, *imageServerPortNum),
			nil
	}
	return readString(filepath.Join(*tftpDirectory, "imageserver"), false)
}

func getImage(imageName string, logger log.DebugLogger) (
	string, *image.Image, *srpc.Client, error) {
	if imageName == "" {
		return "", nil, nil, nil
	}
	imageServerAddress, err := getImageserverAddress()
	if err != nil {
		return "", nil, nil, err
	}
	logger.Printf("dialing imageserver: %s\n", imageServerAddress)
	startTime := time.Now()
	client, err := srpc.DialHTTP("tcp", imageServerAddress, time.Second*15)
	if err != nil {
		return "", nil, nil, err
	}
	logger.Printf("dialed imageserver after: %s\n",
		format.Duration(time.Since(startTime)))
	if err := client.SetKeepAlivePeriod(5 * time.Second); err != nil {
		return "", nil, nil, err
	}
	imageName, img, err := getImageFromClient(client, imageName, false, logger)
	if err != nil {
		client.Close()
		return "", nil, nil, err
	}
	return imageName, img, client, nil
}

func getImageFromClient(client *srpc.Client, imageName string,
	ignoreMissing bool, logger log.DebugLogger) (string, *image.Image, error) {
	startTime := time.Now()
	img, err := imageclient.GetImage(client, imageName)
	if err != nil {
		return "", nil, err
	}
	if img != nil {
		logger.Debugf(0, "got image: %s in %s\n",
			imageName, format.Duration(time.Since(startTime)))
		return imageName, img, nil
	}
	streamName := imageName
	isDir, err := imageclient.CheckDirectory(client, streamName)
	if err != nil {
		return "", nil, err
	}
	if !isDir {
		streamName = filepath.Dir(streamName)
		isDir, err = imageclient.CheckDirectory(client, streamName)
		if err != nil {
			return "", nil, err
		}
	}
	if !isDir {
		return "", nil, fmt.Errorf("%s is not a directory", streamName)
	}
	imageName, err = imageclient.FindLatestImage(client, streamName, false)
	if err != nil {
		return "", nil, err
	}
	if imageName == "" {
		if ignoreMissing {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("no image found in: %s", streamName)
	}
	startTime = time.Now()
	if img, err := imageclient.GetImage(client, imageName); err != nil {
		return "", nil, err
	} else {
		logger.Debugf(0, "got image: %s in %s\n",
			imageName, format.Duration(time.Since(startTime)))
		return imageName, img, nil
	}
}

func getRandomKey(numBytes uint, logger log.DebugLogger) ([]byte, error) {
	logger.Printf("getting %d random bytes\n", numBytes)
	timer := time.AfterFunc(time.Second, func() {
		logger.Println("getting random data is taking too long")
		logger.Println("mash on the keyboard to add entropy")
	})
	startTime := time.Now()
	buffer := make([]byte, numBytes)
	_, err := rand.Read(buffer)
	timer.Stop()
	if err != nil {
		return nil, err
	} else {
		logger.Printf("read %d bytes of random data after %s\n",
			numBytes, format.Duration(time.Since(startTime)))
		return buffer, nil
	}
}

func installRoot(device string, layout installer_proto.StorageLayout,
	fileSystem *filesystem.FileSystem, objGetter objectserver.ObjectsGetter,
	bootInfo *util.BootInfoType, bootPartition int,
	logger log.DebugLogger) error {
	if *dryRun {
		logger.Debugln(0, "dry run: skipping installing root")
		return nil
	}
	logger.Debugln(0, "unpacking root")
	err := unpackAndMount(*mountPoint, fileSystem, objGetter, false, logger)
	if err != nil {
		return err
	}
	if err := makeBindMount(*tmpRoot, *mountPoint); err != nil {
		return err
	}
	if bootPartition > 0 {
		// Mount the /boot partition and copy files into it, then unmount and
		// mount under the root file-system.
		// This ensures that the bootloader has the files it needs and that the
		// root file-system is fully up-to-date with the image.
		partition := layout.BootDriveLayout[bootPartition-1]
		err := mount(partitionName(device, bootPartition), "/tmpboot",
			partition.FileSystemType.String(), logger)
		if err != nil {
			return err
		}
		if partition.FileSystemType == installer_proto.FileSystemTypeVfat {
			// Only directories and regular files supported on VFAT.
			err = fsutil.CopyFilesTree("/tmpboot",
				filepath.Join(*mountPoint, partition.MountPoint))
		} else {
			err = fsutil.CopyTree("/tmpboot",
				filepath.Join(*mountPoint, partition.MountPoint))
		}
		if err != nil {
			return err
		}
		logger.Debugln(0, "copied boot files into /tmpboot")
		if err := syscall.Unmount("/tmpboot", 0); err != nil {
			return fmt.Errorf("error unmounting: %s: %s", "/tmpboot", err)
		}
		logger.Debugln(0, "unmounted /tmpboot")
		err = mount(partitionName(device, bootPartition),
			filepath.Join(*mountPoint, partition.MountPoint),
			partition.FileSystemType.String(), logger)
		if err != nil {
			return err
		}
	}
	var waiter sync.Mutex
	if layout.UseKexec { // Install target OS kernel while it's still mounted.
		waiter.Lock()
		go func() {
			defer waiter.Unlock()
			startTime := time.Now()
			err := run("kexec", *mountPoint, logger,
				"-l", bootInfo.KernelImageFile,
				"--append="+bootInfo.KernelOptions,
				"--console-serial", "--serial-baud=115200",
				"--initrd="+bootInfo.InitrdImageFile)
			if err != nil {
				logger.Printf("error loading new kernel: %w\n", err)
			} else {
				logger.Printf("loaded kernel in %s\n", time.Since(startTime))
			}
		}()
	}
	defer func() {
		waiter.Lock()
		waiter.Unlock()
	}()
	return util.MakeBootable(fileSystem, device, rootFsLabel, *mountPoint, "",
		true, logger)
}

func installTmpRoot(fileSystem *filesystem.FileSystem,
	objGetter objectserver.ObjectsGetter, logger log.DebugLogger) error {
	if fi, err := os.Stat(*tmpRoot); err == nil {
		if fi.IsDir() {
			logger.Debugln(0, "tmproot already exists, not installing")
			return nil
		}
	}
	if *dryRun {
		logger.Debugln(0, "dry run: skipping unpacking tmproot")
		return nil
	}
	logger.Debugln(0, "unpacking tmproot")
	err := unpackAndMount(*tmpRoot, fileSystem, objGetter, true, logger)
	if err != nil {
		return err
	}
	os.Symlink("/proc/mounts", filepath.Join(*tmpRoot, "etc", "mtab"))
	extraBindMounts := []string{*tftpDirectory}
	if err := makeBindMounts(*tmpRoot, extraBindMounts); err != nil {
		return err
	}
	return nil
}

func listDrives(logger log.DebugLogger) ([]*driveType, error) {
	basedir := filepath.Join(*sysfsDirectory, "class", "block")
	file, err := os.Open(basedir)
	if err != nil {
		return nil, err
	}
	names, err := file.Readdirnames(-1)
	file.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var drives []*driveType
	for _, name := range names {
		dirname := filepath.Join(basedir, name)
		if _, err := os.Stat(filepath.Join(dirname, "partition")); err == nil {
			logger.Debugf(2, "skipping partition: %s\n", name)
			continue
		}
		if _, err := os.Stat(filepath.Join(dirname, "device")); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			logger.Debugf(2, "skipping non-device: %s\n", name)
			continue
		}
		if v, err := readInt(filepath.Join(dirname, "removable")); err != nil {
			return nil, err
		} else if v != 0 {
			logger.Debugf(2, "skipping removable device: %s\n", name)
			continue
		}
		busLocation, err := os.Readlink(dirname)
		if err != nil {
			logger.Println(err)
			continue
		}
		if splitLink := strings.Split(busLocation, "/"); len(splitLink) > 5 {
			busLocation = filepath.Join(splitLink[3 : len(splitLink)-2]...)
		}
		drive := &driveType{
			busLocation: busLocation,
			devpath:     filepath.Join("/dev", name),
			name:        name,
		}
		if val, err := readInt(filepath.Join(dirname, "size")); err != nil {
			return nil, err
		} else if drive.mbr, err = readMbr(drive.devpath); err != nil {
			logger.Debugf(2, "skipping unreadable device: %s\n", name)
		} else {
			logger.Debugf(1, "found: %s %d GiB (%d GB) at %s\n",
				name, val>>21, val<<9/1000000000, busLocation)
			drive.size = val << 9
			drives = append(drives, drive)
		}
	}
	if len(drives) < 1 {
		return nil, fmt.Errorf("no drives found")
	}
	// Sort drives based on their bus location. This is a cheap attempt at
	// stable naming.
	sort.SliceStable(drives, func(left, right int) bool {
		return drives[left].busLocation < drives[right].busLocation
	})
	logger.Debugf(0, "sorted drive list: %v\n", drives)
	return drives, nil
}

func makeBindMount(targetRoot, bindMount string) error {
	target := filepath.Join(targetRoot, bindMount)
	if err := os.MkdirAll(target, fsutil.DirPerms); err != nil {
		return err
	}
	err := syscall.Mount(bindMount, target, "", syscall.MS_BIND, "")
	if err != nil {
		return err
	}
	return nil
}

func makeBindMounts(targetRoot string, bindMounts []string) error {
	for _, bindMount := range bindMounts {
		if err := makeBindMount(targetRoot, bindMount); err != nil {
			return err
		}
	}
	return nil
}

func mount(source string, target string, fstype string,
	logger log.DebugLogger) error {
	if *dryRun {
		logger.Debugf(0, "dry run: skipping mount of %s on %s type=%s\n",
			source, target, fstype)
		return nil
	}
	logger.Debugf(0, "mount %s on %s type=%s\n", source, target, fstype)
	if err := os.MkdirAll(target, fsutil.DirPerms); err != nil {
		return err
	}
	return syscall.Mount(source, target, fstype, 0, "")
}

func partitionName(devpath string, partitionNumber int) string {
	devLeafName := filepath.Base(devpath)
	partitionName := "p" + strconv.FormatInt(int64(partitionNumber), 10)
	_, err := os.Stat(filepath.Join("/sys/class/block", devLeafName,
		devLeafName+partitionName))
	if err == nil {
		return devpath + partitionName
	}
	return devpath + strconv.FormatInt(int64(partitionNumber), 10)
}

func readInt(filename string) (uint64, error) {
	if file, err := os.Open(filename); err != nil {
		return 0, err
	} else {
		defer file.Close()
		var value uint64
		if nVal, err := fmt.Fscanf(file, "%d\n", &value); err != nil {
			return 0, err
		} else if nVal != 1 {
			return 0, fmt.Errorf("read %d values, expected 1", nVal)
		} else {
			return value, nil
		}
	}
}

func remapDevice(device, target string, encrypt bool) string {
	if !encrypt || target == rootMountPoint || target == bootMountPoint {
		return device
	} else {
		return filepath.Join("/dev/mapper", filepath.Base(device))
	}
}

func selectDrives(input []*driveType, logger log.DebugLogger) (
	[]*driveType, error) {
	if *driveSelector == "" {
		logger.Println("selecting all usable drives")
		return input, nil
	}
	names := make([]string, 0, len(input))
	table := make(map[string]*driveType, len(input))
	for _, drive := range input {
		names = append(names, drive.name)
		table[drive.name] = drive
	}
	stderr := &bytes.Buffer{}
	cmd := exec.Command(*driveSelector, names...)
	cmd.Stderr = stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running: %s: %s, output: %s",
			*driveSelector, err, stderr)
	}
	names, err = fsutil.ReadLines(bytes.NewReader(stdout))
	if err != nil {
		return nil, err
	}
	output := make([]*driveType, 0, len(names))
	for _, name := range names {
		if drive := table[name]; drive == nil {
			return nil, fmt.Errorf("cannot select non-existant drive: %s", name)
		} else {
			output = append(output, drive)
			logger.Printf("selected drive: %s\n", name)
		}
	}
	return output, nil
}

func unmountStorage(logger log.DebugLogger) error {
	if *dryRun {
		logger.Debugln(0, "dry run: skipping unmounting")
		return nil
	}
	err := syscall.Unmount(filepath.Join(*tmpRoot, *mountPoint), 0)
	if err != nil {
		return err
	}
	syscall.Sync()
	time.Sleep(time.Millisecond * 100)
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return err
	}
	defer file.Close()
	var mountPoints []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		} else {
			if strings.HasPrefix(fields[1], *mountPoint) {
				mountPoints = append(mountPoints, fields[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	unmountedMainMountPoint := false
	for index := len(mountPoints) - 1; index >= 0; index-- {
		mntPoint := mountPoints[index]
		if mntPoint == *mountPoint {
			if err := closeEncryptedVolumes(logger); err != nil {
				return err
			}
		}
		if err := syscall.Unmount(mntPoint, 0); err != nil {
			return fmt.Errorf("error unmounting: %s: %s", mntPoint, err)
		} else {
			logger.Debugf(2, "unmounted: %s\n", mntPoint)
		}
		if mntPoint == *mountPoint {
			unmountedMainMountPoint = true
		}
	}
	if !unmountedMainMountPoint {
		return errors.New("did not find main mount point to unmount")
	}
	syscall.Sync()
	return nil
}

func (drive driveType) String() string {
	return drive.name
}

func (drive driveType) cryptSetup(cpuSharer cpusharer.CpuSharer, device string,
	logger log.DebugLogger) error {
	cpuSharer.GrabCpu()
	defer cpuSharer.ReleaseCpu()
	startTime := time.Now()
	err := run("cryptsetup", *tmpRoot, logger, "--verbose",
		"--key-file", keyFile,
		"--cipher", "aes-xts-plain64", "--key-size", "512",
		"--hash", "sha512", "--iter-time", "5000", "--use-urandom",
		"luksFormat", device)
	if err != nil {
		return err
	}
	logger.Printf("formatted encrypted device %s in %s\n",
		device, time.Since(startTime))
	startTime = time.Now()
	if drive.discarded {
		err = run("cryptsetup", *tmpRoot, logger, "open", "--type", "luks",
			"--allow-discards",
			"--key-file", keyFile, device, filepath.Base(device))
	} else {
		err = run("cryptsetup", *tmpRoot, logger, "open", "--type", "luks",
			"--key-file", keyFile, device, filepath.Base(device))
	}
	if err != nil {
		return err
	}
	logger.Printf("opened encrypted device %s in %s\n",
		device, time.Since(startTime))
	return nil
}

func (drive driveType) makeFileSystem(cpuSharer cpusharer.CpuSharer,
	device, target string, fstype installer_proto.FileSystemType, encrypt bool,
	mkfsMutex *sync.Mutex, bytesPerInode uint, logger log.DebugLogger) error {
	startTime := time.Now()
	numIterations, numOpened, err := fsutil.WaitForBlockAvailable(device,
		5*time.Second)
	if err != nil {
		return err
	}
	if numIterations > 0 {
		logger.Debugf(0, "%s available after %d iterations, %d opens, %s\n",
			device, numIterations, numOpened,
			format.Duration(time.Since(startTime)))
	}
	label := target
	erase := !drive.discarded
	if label == rootMountPoint {
		label = rootFsLabel
	} else if label == bootMountPoint {
		label = bootFsLabel
	} else if label == efiMountPoint {
		label = efiFsLabel
	} else if encrypt {
		if err := drive.cryptSetup(cpuSharer, device, logger); err != nil {
			return err
		}
		device = filepath.Join("/dev/mapper", filepath.Base(device))
		erase = true
	}
	if erase {
		if err := eraseStart(device, logger); err != nil {
			return err
		}
	}
	if mkfsMutex != nil {
		mkfsMutex.Lock()
	}
	startTime = time.Now()
	switch fstype {
	case installer_proto.FileSystemTypeExt4:
		if bytesPerInode > 0 {
			err = run("mkfs.ext4", *tmpRoot, logger,
				"-i", strconv.Itoa(int(bytesPerInode)), "-L", label,
				"-E", "lazy_itable_init=0,lazy_journal_init=0", device)
		} else {
			err = run("mkfs.ext4", *tmpRoot, logger, "-L", label,
				"-E", "lazy_itable_init=0,lazy_journal_init=0", device)
		}
	case installer_proto.FileSystemTypeVfat:
		err = run("mkfs.vfat", *tmpRoot, logger, "--codepage=437",
			"-n", label, device)
	default:
		return fmt.Errorf("unsupported file-system type: %d (%s)",
			fstype, fstype)
	}
	if mkfsMutex != nil {
		mkfsMutex.Unlock()
	}
	if err != nil {
		return err
	}
	logger.Printf("made file-system on %s in %s\n",
		device, time.Since(startTime))
	return nil
}

func (drive driveType) writeDeviceEntries(device, target string,
	fstype installer_proto.FileSystemType,
	fsTab, cryptTab io.Writer, checkOrder uint) error {
	label := target
	if label == rootMountPoint {
		label = rootFsLabel
	} else if label == bootMountPoint {
		label = bootFsLabel
	} else if label == efiMountPoint {
		label = efiFsLabel
	} else {
		var options string
		if drive.discarded {
			options = "discard"
		}
		_, err := fmt.Fprintf(cryptTab, "%-15s %-23s %-15s %s\n",
			filepath.Base(device), device, keyFile, options)
		if err != nil {
			return err
		}
	}
	var fsFlags string
	if drive.discarded {
		fsFlags = "discard"
	}
	if label == "EFI" {
		fsFlags = "noauto"
	}
	return util.WriteFstabEntry(fsTab, "LABEL="+label, target, fstype.String(),
		fsFlags, 0, checkOrder)
}

func (rebooter kexecRebooter) Reboot() error {
	return run("kexec", *tmpRoot, rebooter.logger, "-e")
}

func (rebooter kexecRebooter) String() string {
	return "kexec"
}
