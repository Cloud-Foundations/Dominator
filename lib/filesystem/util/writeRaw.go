package util

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/backoffdelay"
	"github.com/Cloud-Foundations/Dominator/lib/cleanup"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil/mounts"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
	"github.com/Cloud-Foundations/Dominator/proto/installer"
)

const (
	BLKGETSIZE  = 0x00001260
	createFlags = os.O_CREATE | os.O_TRUNC | os.O_RDWR
)

var (
	mutex               sync.Mutex
	defaultMkfsFeatures map[string]struct{} // Key: feature name.
	grubTemplateDos     = template.Must(template.New("grub").Parse(
		grubTemplateStringDos))
	grubTemplateEfi = template.Must(template.New("grub").Parse(
		grubTemplateStringEfi))
)

func checkIfPartition(device string) (bool, error) {
	if isBlock, err := checkIsBlock(device); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	} else if !isBlock {
		return false, fmt.Errorf("%s is not a block device", device)
	} else {
		return true, nil
	}
}

func checkIsBlock(filename string) (bool, error) {
	if fi, err := os.Stat(filename); err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("error stating: %s: %s", filename, err)
		}
		return false, err
	} else {
		return fi.Mode()&os.ModeDevice == os.ModeDevice, nil
	}
}

func findExecutable(rootDir, file string) error {
	if d, err := os.Stat(filepath.Join(rootDir, file)); err != nil {
		return err
	} else {
		if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
			return nil
		}
		return os.ErrPermission
	}
}

func getBootDirectory(fs *filesystem.FileSystem) (
	*filesystem.DirectoryInode, error) {
	if fs.EntriesByName == nil {
		fs.BuildEntryMap()
	}
	dirent, ok := fs.EntriesByName["boot"]
	if !ok {
		return nil, errors.New("missing /boot directory")
	}
	bootDirectory, ok := dirent.Inode().(*filesystem.DirectoryInode)
	if !ok {
		return nil, errors.New("/boot is not a directory")
	}
	return bootDirectory, nil
}

func getDefaultMkfsFeatures(device, size string, logger log.Logger) (
	map[string]struct{}, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if defaultMkfsFeatures == nil {
		startTime := time.Now()
		logger.Println("Making calibration file-system")
		cmd := exec.Command("mkfs.ext4", "-L", "calibration-fs", "-i", "65536",
			device, size)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf(
				"error making calibration file-system on: %s: %s: %s",
				device, err, output)
		}
		logger.Printf("Made calibration file-system in %s\n",
			format.Duration(time.Since(startTime)))
		cmd = exec.Command("dumpe2fs", "-h", device)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("error dumping file-system info: %s: %s",
				err, output)
		}
		defaultMkfsFeatures = make(map[string]struct{})
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			if fields[0] != "Filesystem" || fields[1] != "features:" {
				continue
			}
			for _, field := range fields[2:] {
				defaultMkfsFeatures[field] = struct{}{}
			}
			break
		}
		// Scrub out the calibration file-system.
		buffer := make([]byte, 65536)
		if file, err := os.OpenFile(device, os.O_WRONLY, 0); err == nil {
			file.Write(buffer)
			file.Close()
		}
	}
	return defaultMkfsFeatures, nil
}

func getUnsupportedOptions(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter) ([]string, error) {
	bootDirectory, err := getBootDirectory(fs)
	if err != nil {
		return nil, err
	}
	dirent, ok := bootDirectory.EntriesByName["ext4.unsupported-features"]
	var unsupportedOptions []string
	if ok {
		if inode, ok := dirent.Inode().(*filesystem.RegularInode); ok {
			hashes := []hash.Hash{inode.Hash}
			objectsReader, err := objectsGetter.GetObjects(hashes)
			if err != nil {
				return nil, err
			}
			defer objectsReader.Close()
			size, reader, err := objectsReader.NextObject()
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			if size > 1024 {
				return nil,
					errors.New("/boot/ext4.unsupported-features is too large")
			}
			for {
				var option string
				_, err := fmt.Fscanf(reader, "%s\n", &option)
				if err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				} else {
					unsupportedOptions = append(unsupportedOptions,
						strings.Map(sanitiseInput, option))
				}
			}
		}
	}
	return unsupportedOptions, nil
}

func getRootPartition(bootDevice, index string) (string, error) {
	rootPartition := bootDevice + "p" + index
	if isPartition, err := checkIfPartition(rootPartition); err != nil {
		return "", err
	} else if isPartition {
		return rootPartition, nil
	}
	rootPartition = bootDevice + index
	if isPartition, err := checkIfPartition(rootPartition); err != nil {
		return "", err
	} else if !isPartition {
		return "", errors.New("no root partition found")
	} else {
		return rootPartition, nil
	}
}

func lookPath(rootDir, file string) (string, error) {
	if strings.Contains(file, "/") {
		if err := findExecutable(rootDir, file); err != nil {
			return "", err
		}
		return file, nil
	}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "." // Unix shell semantics: path element "" means "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(rootDir, path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("(chroot=%s) %s not found in PATH", rootDir, file)
}

func makeAndWriteRoot(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter, bootDevice, rootDevice string,
	makeEfi bool, options WriteRawOptions, partitionPrefix string,
	startPartition int, cancel <-chan os.Signal, logger log.DebugLogger) error {
	unsupportedOptions, err := getUnsupportedOptions(fs, objectsGetter)
	if err != nil {
		return err
	}
	var efiDevice string
	if options.InstallBootloader {
		if makeEfi {
			efiDevice = string([]byte(rootDevice[:len(rootDevice)-1])) + "1"
			if err := makeEfiFs(efiDevice, "EFI", logger); err != nil {
				return err
			}
		}
	}
	var bootInfo *BootInfoType
	if options.RootLabel == "" {
		options.RootLabel = fmt.Sprintf("rootfs@%x", time.Now().Unix())
	}
	if options.InstallBootloader {
		var err error
		kernelOptions := []string{"net.ifnames=0"}
		if options.ExtraKernelOptions != "" {
			kernelOptions = append(kernelOptions, options.ExtraKernelOptions)
		}
		kernelOptionsString := strings.Join(kernelOptions, " ")
		bootInfo, err = getBootInfo(fs, MakeKernelOptionsParams{
			ArchitectureType: options.ArchitectureType,
			ExtraOptions:     kernelOptionsString,
			RootDevice:       "LABEL=" + options.RootLabel,
		})
		if err != nil {
			return err
		}
	}
	err = MakeExt4fsWithParams(rootDevice, MakeExt4fsParams{
		BytesPerInode:      8192,
		Label:              options.RootLabel,
		NoDiscard:          options.AllocateBlocks,
		UnsupportedOptions: unsupportedOptions,
	},
		logger)
	if err != nil {
		return err
	}
	err = makeExtraFileSystems(bootDevice, partitionPrefix, startPartition,
		options, logger)
	if err != nil {
		return err
	}
	rootMount, err := ioutil.TempDir("", "write-raw-image")
	if err != nil {
		return err
	}
	defer os.RemoveAll(rootMount)
	err = wsyscall.Mount(rootDevice, rootMount, "ext4", 0, "")
	if err != nil {
		return fmt.Errorf("error mounting: %s", rootDevice)
	}
	cleanupFunctions := cleanup.NewCleanupFunctions(logger)
	cleanupFunctions.Add(func() error {
		return wsyscall.Unmount(rootMount, 0)
	})
	doUnmount := true
	defer func() {
		if doUnmount {
			cleanupFunctions.HardCleanup()
		}
	}()
	os.RemoveAll(filepath.Join(rootMount, "lost+found"))
	for index, partition := range options.ExtraPartitions {
		device := fmt.Sprintf("%s%s%d",
			bootDevice, partitionPrefix, startPartition+index)
		mountPoint := filepath.Join(rootMount,
			filepath.Clean(partition.MountPoint))
		if err := os.MkdirAll(mountPoint, fsutil.DirPerms); err != nil {
			return err
		}
		err := wsyscall.Mount(device, mountPoint, "ext4", 0, "")
		if err != nil {
			return err
		}
		cleanupFunctions.Add(func() error {
			return wsyscall.Unmount(mountPoint, 0)
		})
	}
	if err := unpack(fs, objectsGetter, rootMount, cancel, logger); err != nil {
		return err
	}
	for _, dirname := range options.OverlayDirectories {
		dirname := filepath.Clean(dirname) // Stop funny business.
		pathname := filepath.Join(rootMount, dirname)
		if err := os.MkdirAll(pathname, fsutil.DirPerms); err != nil {
			return err
		}
		// Set permissions for what are likely to be mount points.
		if err := os.Lchown(pathname, 0, 0); err != nil {
			return err
		}
		if err := os.Chmod(pathname, fsutil.DirPerms); err != nil {
			return err
		}
	}
	if err := writeOverlayFiles(rootMount, options.OverlayFiles); err != nil {
		return err
	}
	if err := writeImageName(rootMount, options.InitialImageName); err != nil {
		return err
	}
	if options.WriteFstab {
		err := writeRootFstabEntry(rootMount, options.RootLabel)
		if err != nil {
			return err
		}
	}
	if options.InstallBootloader {
		var efiMount string
		if efiDevice != "" {
			efiMount = filepath.Join(rootMount, "/mnt/efi")
			if err := os.MkdirAll(efiMount, fsutil.DirPerms); err != nil {
				return err
			}
			err := wsyscall.Mount(efiDevice, efiMount, "vfat", 0, "")
			if err != nil {
				return err
			}
		}
		err := bootInfo.installBootloader(bootDevice, rootMount,
			options.DoChroot, logger)
		if err != nil {
			if efiMount != "" {
				wsyscall.Unmount(efiMount, 0)
			}
			return err
		}
		if efiMount != "" {
			if err := wsyscall.Unmount(efiMount, 0); err != nil {
				return err
			}
		}
	}
	doUnmount = false
	startTime := time.Now()
	if err := cleanupFunctions.HardCleanup(); err != nil {
		return fmt.Errorf("cleanup error: %w", err)
	}
	if timeTaken := time.Since(startTime); timeTaken > 10*time.Millisecond {
		logger.Debugf(0, "Unmounted in %s\n",
			format.Duration(time.Since(startTime)))
	}
	return nil
}

func makeBootable(params MakeBootableParams) error {
	err := writeRootFstabEntry(params.RootDirectory, params.RootLabel)
	if err != nil {
		return err
	}
	bootInfo, err := getBootInfo(params.FileSystem, MakeKernelOptionsParams{
		ArchitectureType: params.ArchitectureType,
		ExtraOptions:     params.KernelOptions,
		RootDevice:       "LABEL=" + params.RootLabel,
	})
	if err != nil {
		return err
	} else {
		return bootInfo.installBootloader(params.DeviceName,
			params.RootDirectory, params.DoChroot, params.Logger)
	}
}

func makeEfiFs(deviceName, label string, logger log.Logger) error {
	cmd := exec.Command("mkfs.vfat", "--codepage=437",
		"-F", "32", "-n", label, deviceName)
	startTime := time.Now()
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error making file-system on: %s: %s: %s",
			deviceName, err, output)
	}
	logger.Printf("Made VFAT file-system on: %s in %s\n",
		deviceName, format.Duration(time.Since(startTime)))
	return nil
}

func makeExt4fs(deviceName string, params MakeExt4fsParams,
	logger log.Logger) error {
	if params.Size < 1 {
		var err error
		params.Size, err = wsyscall.GetDeviceSize(deviceName)
		if err != nil {
			return err
		}
	}
	sizeString := strconv.FormatUint(params.Size>>10, 10)
	var featureOptions []string
	if len(params.UnsupportedOptions) > 0 {
		defaultFeatures, err := getDefaultMkfsFeatures(deviceName, sizeString,
			logger)
		if err != nil {
			return err
		}
		for _, option := range params.UnsupportedOptions {
			if _, ok := defaultFeatures[option]; ok {
				featureOptions = append(featureOptions, "^"+option)
			}
		}
	}
	cmd := exec.Command("mkfs.ext4")
	if params.BytesPerInode != 0 {
		cmd.Args = append(cmd.Args, "-i",
			strconv.FormatUint(params.BytesPerInode, 10))
	}
	if params.Label != "" {
		cmd.Args = append(cmd.Args, "-L", params.Label)
	}
	if params.ReservedBlocksPercentage != 0 {
		cmd.Args = append(cmd.Args, "-m",
			strconv.FormatUint(uint64(params.ReservedBlocksPercentage), 10))
	}
	var extendedOptions []string
	if params.NoDiscard {
		extendedOptions = append(extendedOptions, "nodiscard")
	}
	if params.RootGroupId != 0 || params.RootUserId != 0 {
		extendedOptions = append(extendedOptions,
			fmt.Sprintf("root_owner=%d:%d",
				params.RootUserId, params.RootGroupId))
	}
	if len(extendedOptions) > 0 {
		cmd.Args = append(cmd.Args, "-E", strings.Join(extendedOptions, ","))
	}
	if len(featureOptions) > 0 {
		cmd.Args = append(cmd.Args, "-O", strings.Join(featureOptions, ","))
	}
	cmd.Args = append(cmd.Args, deviceName, sizeString)
	startTime := time.Now()
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error making file-system on: %s: %s: %s",
			deviceName, err, output)
	}
	logger.Printf("Made %s Ext4 file-system on: %s in %s\n",
		format.FormatBytes(params.Size), deviceName,
		format.Duration(time.Since(startTime)))
	return nil
}

func makeExtraFileSystems(bootDevice, partitionPrefix string,
	startPartition int, options WriteRawOptions, logger log.Logger) error {
	for index, partition := range options.ExtraPartitions {
		if partition.FileSystemType != installer.FileSystemTypeExt4 {
			return fmt.Errorf("extra partition: %d is not ext4fs", index)
		}
		err := makeExt4fs(fmt.Sprintf("%s%s%d",
			bootDevice, partitionPrefix, startPartition+index),
			MakeExt4fsParams{
				BytesPerInode:            uint64(partition.BytesPerInode),
				Label:                    partition.FileSystemLabel,
				NoDiscard:                true,
				ReservedBlocksPercentage: uint16(partition.ReservedBlocksPercentage),
				RootGroupId:              partition.RootGroupId,
				RootUserId:               partition.RootUserId,
				Size:                     partition.MinimumBytes,
			},
			logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeKernelOptions(params MakeKernelOptionsParams) string {
	var serialPort string
	switch params.ArchitectureType {
	case hypervisor.ArchitectureTypeAmd64, hypervisor.ArchitectureTypeAuto:
		serialPort = "ttyS0"
	case hypervisor.ArchitectureTypeArm64:
		serialPort = "ttyAMA0"
	}
	return fmt.Sprintf("root=%s ro console=tty0 console=%s,115200n8 %s",
		params.RootDevice, serialPort, params.ExtraOptions)
}

func sanitiseInput(ch rune) rune {
	if 'a' <= ch && ch <= 'z' {
		return ch
	} else if '0' <= ch && ch <= '9' {
		return ch
	} else if ch == '_' {
		return ch
	} else {
		return -1
	}
}

func writeOverlayFiles(mountPoint string,
	overlayFiles map[string][]byte) error {
	for filename, data := range overlayFiles {
		filename := filepath.Clean(filename) // Stop funny business.
		err := writeFile(filepath.Join(mountPoint, filename), data)
		if err != nil {
			return err
		}
	}
	return nil
}

func getBootInfo(fs *filesystem.FileSystem, params MakeKernelOptionsParams) (
	*BootInfoType, error) {
	bootDirectory, err := getBootDirectory(fs)
	if err != nil {
		return nil, err
	}
	bootInfo := &BootInfoType{
		ArchitectureType: params.ArchitectureType,
		BootDirectory:    bootDirectory,
		KernelOptions:    MakeKernelOptionsWithParams(params),
	}
	for _, dirent := range bootDirectory.EntryList {
		if strings.HasPrefix(dirent.Name, "initrd.img-") ||
			strings.HasPrefix(dirent.Name, "initramfs-") {
			if bootInfo.InitrdImageFile != "" {
				return nil, errors.New("multiple initrd images")
			}
			bootInfo.InitrdImageDirent = dirent
			bootInfo.InitrdImageFile = "/boot/" + dirent.Name
		}
		if strings.HasPrefix(dirent.Name, "vmlinuz-") {
			if bootInfo.KernelImageFile != "" {
				return nil, errors.New("multiple kernel images")
			}
			bootInfo.KernelImageDirent = dirent
			bootInfo.KernelImageFile = "/boot/" + dirent.Name
		}
	}
	return bootInfo, nil
}

func (bootInfo *BootInfoType) installBootloader(deviceName string,
	rootDir string, doChroot bool, logger log.DebugLogger) error {
	startTime := time.Now()
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return err
	}
	var bootDir, chrootDir, efiDir string
	var efiEntry *mounts.MountEntry
	if doChroot {
		bootDir = "/boot"
		chrootDir = rootDir
		efiDir = "/mnt/efi"
		efiEntry = mountTable.FindEntry(filepath.Join(rootDir, efiDir))
	} else {
		bootDir = filepath.Join(rootDir, "boot")
		efiDir = filepath.Join(rootDir, "/mnt/efi")
		efiEntry = mountTable.FindEntry(efiDir)
	}
	if efiEntry != nil && efiEntry.Type != "vfat" {
		efiEntry = nil
	}
	grubConfigFile := filepath.Join(rootDir, "boot", "grub", "grub.cfg")
	bootEntry := mountTable.FindEntry(grubConfigFile)
	if efiEntry == nil && bootEntry != nil && bootEntry.Type == "vfat" {
		efiDir = bootDir
		efiEntry = bootEntry
	}
	var isGrub2 bool
	grubInstaller, err := lookPath(chrootDir, "grub-install")
	if err != nil {
		grubInstaller, err = lookPath(chrootDir, "grub2-install")
		if err != nil {
			return fmt.Errorf("cannot find GRUB installer: %s", err)
		}
		grubConfigFile = filepath.Join(rootDir, "boot", "grub2", "grub.cfg")
		isGrub2 = true
	}
	cmd := exec.Command(grubInstaller,
		"--boot-directory="+bootDir,
		deviceName)
	if efiEntry != nil {
		cmd.Args = append(cmd.Args,
			"--efi-directory="+efiDir,
			"--removable", // Needed for loop devices.
		)
		switch bootInfo.ArchitectureType {
		case hypervisor.ArchitectureTypeAmd64:
			cmd.Args = append(cmd.Args, "--target=x86_64-efi")
		case hypervisor.ArchitectureTypeArm64:
			cmd.Args = append(cmd.Args, "--target=arm64-efi")
		default:
			return fmt.Errorf("unsupported GRUB EFI target for arch: %s",
				bootInfo.ArchitectureType)
		}
		if isGrub2 {
			// Likely RedHat or derivative: work around their controlling
			// behaviour.
			cmd.Args = append(cmd.Args,
				"--force",
			)
		}
	} else {
		switch bootInfo.ArchitectureType {
		case hypervisor.ArchitectureTypeAmd64:
			cmd.Args = append(cmd.Args, "--target=i386-pc")
		default:
			return fmt.Errorf("unsupported GRUB target for arch: %s",
				bootInfo.ArchitectureType)
		}
	}
	if doChroot {
		cmd.Dir = "/"
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		err := wsyscall.SetSysProcAttrChroot(cmd.SysProcAttr, chrootDir)
		if err != nil {
			return fmt.Errorf("error setting chroot: %s", err)
		}
		logger.Debugf(0, "running(chroot=%s): %s %s\n",
			chrootDir, cmd.Path, strings.Join(cmd.Args[1:], " "))
	} else {
		logger.Debugf(0, "running: %s %s\n",
			cmd.Path, strings.Join(cmd.Args[1:], " "))
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error installing GRUB on: %s: %s: %s",
			deviceName, err, output)
	}
	logger.Printf("installed GRUB in %s\n",
		format.Duration(time.Since(startTime)))
	return bootInfo.writeGrubConfigAndTemplate(rootDir, grubConfigFile,
		mountTable, efiEntry != nil)
}

func (bootInfo *BootInfoType) writeGrubConfig(filename string,
	isEfi bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating GRUB config file: %s", err)
	}
	defer file.Close()
	var tmpl *template.Template
	if isEfi {
		tmpl = grubTemplateEfi
	} else {
		tmpl = grubTemplateDos
	}
	if err := tmpl.Execute(file, bootInfo); err != nil {
		return err
	}
	return file.Close()
}

func (bootInfo *BootInfoType) writeGrubConfigAndTemplate(rootDir string,
	grubConfigFile string, mountTable *mounts.MountTable, isEfi bool) error {
	bootEntry := mountTable.FindEntry(grubConfigFile)
	rootEntry := mountTable.FindEntry(rootDir)
	if bootEntry != rootEntry { // "/boot" directory in a separate file-system.
		newBootInfo := *bootInfo
		newBootInfo.InitrdImageFile = "/" + newBootInfo.InitrdImageDirent.Name
		newBootInfo.KernelImageFile = "/" + newBootInfo.KernelImageDirent.Name
		bootInfo = &newBootInfo
	}
	if err := bootInfo.writeGrubConfig(grubConfigFile, isEfi); err != nil {
		return err
	}
	return bootInfo.writeGrubTemplate(grubConfigFile+".template", isEfi)
}

func (bootInfo *BootInfoType) writeGrubTemplate(filename string,
	isEfi bool) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating GRUB config file template: %s", err)
	}
	var data string
	if isEfi {
		data = grubTemplateStringEfi
	} else {
		data = grubTemplateStringDos
	}
	defer file.Close()
	if _, err := file.Write([]byte(data)); err != nil {
		return err
	}
	return file.Close()
}

func (bootInfo *BootInfoType) writeBootloaderConfig(rootDir string,
	logger log.Logger) error {
	mountTable, err := mounts.GetMountTable()
	if err != nil {
		return err
	}
	grubConfigFile := filepath.Join(rootDir, "boot", "grub", "grub.cfg")
	_, err = lookPath("", "grub-install")
	if err != nil {
		_, err = lookPath("", "grub2-install")
		if err != nil {
			return fmt.Errorf("cannot find GRUB installer: %s", err)
		}
		grubConfigFile = filepath.Join(rootDir, "boot", "grub2", "grub.cfg")
	}
	bootEntry := mountTable.FindEntry(grubConfigFile)
	var isEfi bool
	if bootEntry != nil && bootEntry.Type == "vfat" {
		isEfi = true
	}
	return bootInfo.writeGrubConfigAndTemplate(rootDir, grubConfigFile,
		mountTable, isEfi)
}

func waitForRootPartition(bootDevice, index string, timeout time.Duration) (
	string, error) {
	sleeper := backoffdelay.NewExponential(time.Millisecond,
		100*time.Millisecond, 2)
	stopTime := time.Now().Add(timeout)
	for time.Until(stopTime) >= 0 {
		if partition, err := getRootPartition(bootDevice, index); err == nil {
			return partition, nil
		}
		sleeper.Sleep()
	}
	return "", errors.New("timed out waiting for root partition")
}

func writeFile(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Close()
}

func writeFstabEntry(writer io.Writer,
	source, mountPoint, fileSystemType, flags string,
	dumpFrequency, checkOrder uint) error {
	if flags == "" {
		flags = "defaults"
	}
	_, err := fmt.Fprintf(writer, "%-22s %-10s %-5s %-10s %d %d\n",
		source, mountPoint, fileSystemType, flags, dumpFrequency, checkOrder)
	return err
}

func writeImageName(mountPoint, imageName string) error {
	pathname := filepath.Join(mountPoint, constants.InitialImageNameFile)
	if imageName == "" {
		if err := os.Remove(pathname); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(pathname), fsutil.DirPerms); err != nil {
		return err
	}
	buffer := &bytes.Buffer{}
	fmt.Fprintln(buffer, imageName)
	return fsutil.CopyToFile(pathname, fsutil.PublicFilePerms, buffer, 0)
}

func writeMbr(filename string, tableType mbr.TableType, rootSize uint64,
	options WriteRawOptions) error {
	if len(options.ExtraPartitions) < 1 {
		return mbr.WriteDefault(filename, tableType)
	}
	partitions := []mbr.Partition{{Size: types.Bytes(rootSize), Type: "ext2"}}
	for _, extraPartition := range options.ExtraPartitions {
		partitions = append(partitions, mbr.Partition{
			Size: types.Bytes(extraPartition.MinimumBytes),
			Type: extraPartition.FileSystemType.String(),
		})
	}
	return mbr.Write(filename, tableType, partitions)
}

func writeToBlock(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter, bootDevice string,
	tableType mbr.TableType, options WriteRawOptions,
	logger log.DebugLogger) error {
	usageEstimate := fs.EstimateUsage(0)
	rootBytes := usageEstimate + usageEstimate>>3 // 12% extra for good luck.
	if options.MinimumFreeBytes < 1 &&
		rootBytes >= uint64(options.MinimumBytes) {
		return fmt.Errorf("root file-system too large")
	}
	rootBytes += options.MinimumFreeBytes
	if options.MinimumBytes > types.Bytes(rootBytes) {
		rootBytes = uint64(options.MinimumBytes)
	}
	if err := writeMbr(bootDevice, tableType, rootBytes, options); err != nil {
		return err
	}
	index := 1
	indexString := "1"
	var makeEfi bool
	if tableType == mbr.TABLE_TYPE_GPT {
		index = 2
		indexString = "2"
		makeEfi = true
	}
	rootDevice, err := waitForRootPartition(bootDevice, indexString,
		options.PartitionWaitTimeout)
	if err != nil {
		return err
	}
	err = makeAndWriteRoot(fs, objectsGetter, bootDevice, rootDevice, makeEfi,
		options, "", index+1, nil, logger)
	if err != nil {
		return err
	}
	return nil
}

func writeToFile(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter, rawFilename string,
	perm os.FileMode, tableType mbr.TableType, options WriteRawOptions,
	logger log.DebugLogger) error {
	tmpFilename := rawFilename + "~"
	if file, err := os.OpenFile(tmpFilename, createFlags, perm); err != nil {
		return err
	} else {
		file.Close()
		defer os.Remove(tmpFilename)
	}
	usageEstimate := fs.EstimateUsage(0)
	rootBytes := usageEstimate + usageEstimate>>3 // 12% extra for good luck.
	if options.MinimumFreeBytes < 1 &&
		rootBytes >= uint64(options.MinimumBytes) {
		return fmt.Errorf("root file-system too large")
	}
	rootBytes += options.MinimumFreeBytes
	if options.MinimumBytes > types.Bytes(rootBytes) {
		rootBytes = uint64(options.MinimumBytes)
	}
	minBytes := rootBytes + 1<<20 // Leave bottom 1 MiB free.
	if tableType == mbr.TABLE_TYPE_GPT {
		minBytes += 130 << 20 // Leave space for the EFI partition.
	}
	for _, extraPartition := range options.ExtraPartitions {
		minBytes += extraPartition.MinimumBytes
	}
	if options.RoundupPower < 24 {
		options.RoundupPower = 24 // 16 MiB.
	}
	imageUnits := minBytes >> options.RoundupPower
	if imageUnits<<options.RoundupPower < minBytes {
		imageUnits++
	}
	imageSize := imageUnits << options.RoundupPower
	if err := os.Truncate(tmpFilename, int64(imageSize)); err != nil {
		return err
	}
	if options.AllocateBlocks {
		err := fsutil.FallocateOrFill(tmpFilename, imageSize, logger)
		if err != nil {
			return fmt.Errorf("error fallocating file: %s: %s",
				tmpFilename, err)
		}
	}
	if err := writeMbr(tmpFilename, tableType, rootBytes, options); err != nil {
		return err
	}
	partition := 1
	partitionString := "p1"
	var makeEfi bool
	if tableType == mbr.TABLE_TYPE_GPT {
		makeEfi = true
		partition = 2
		partitionString = "p2"
	}
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, syscall.SIGINT)
	defer signal.Stop(cancel)
	loopDevice, err := fsutil.LoopbackSetupAndWaitForPartition(tmpFilename,
		partitionString, time.Minute, logger)
	if err != nil {
		return err
	}
	defer fsutil.LoopbackDeleteAndWaitForPartition(loopDevice, partitionString,
		time.Minute, logger)
	rootDevice := loopDevice + partitionString
	err = makeAndWriteRoot(fs, objectsGetter, loopDevice, rootDevice, makeEfi,
		options, "p", partition+1, cancel, logger)
	if err != nil {
		return err
	}
	select {
	case <-cancel:
	default:
	}
	return os.Rename(tmpFilename, rawFilename)
}

func writeRaw(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter, rawFilename string,
	perm os.FileMode, tableType mbr.TableType, options WriteRawOptions,
	logger log.DebugLogger) error {
	if options.ArchitectureType == hypervisor.ArchitectureTypeAuto {
		options.ArchitectureType = hypervisor.ArchitectureTypeRuntime
	}
	if options.PartitionWaitTimeout < time.Millisecond {
		options.PartitionWaitTimeout = 2 * time.Second
	}
	if isBlock, err := checkIsBlock(rawFilename); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if isBlock {
		return writeToBlock(fs, objectsGetter, rawFilename, tableType,
			options, logger)
	}
	return writeToFile(fs, objectsGetter, rawFilename, perm, tableType,
		options, logger)
}

func writeRootFstabEntry(rootDir, rootLabel string) error {
	pathname := filepath.Join(rootDir, "etc", "fstab")
	oldFstab, err := ioutil.ReadFile(pathname)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := os.Create(pathname)
	if err != nil {
		return err
	} else {
		doClose := true
		defer func() {
			if doClose {
				file.Close()
			}
		}()
		w := bufio.NewWriter(file)
		err := writeFstabEntry(w, "LABEL="+rootLabel, "/", "ext4", "", 0, 1)
		if err != nil {
			return err
		}
		if _, err := w.Write(oldFstab); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
		doClose = false
		return file.Close()
	}
}

const grubTemplateStringDos string = `# Generated from simple template.
insmod serial
serial --unit=0 --speed=115200
terminal_output serial
set timeout=0

menuentry 'Linux' 'Solitary Linux' {
        insmod gzio
        insmod part_msdos
        insmod ext2
        echo    'Loading Linux {{.KernelImageFile}} ...'
        linux   {{.KernelImageFile}} {{.KernelOptions}}
        echo    'Loading initial ramdisk ...'
        initrd  {{.InitrdImageFile}}
}
`

const grubTemplateStringEfi string = `# Generated from simple template.
insmod serial
serial --unit=0 --speed=115200
terminal_input  serial console
terminal_output serial console
set timeout=2

menuentry 'Linux' 'Solitary Linux' {
        insmod efi_gop
        insmod gzio
        insmod part_gpt
        insmod fat
        echo    'Loading Linux {{.KernelImageFile}} ...'
        linux   {{.KernelImageFile}} {{.KernelOptions}}
        echo    'Loading initial ramdisk ...'
        initrd  {{.InitrdImageFile}}
}
`
