package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func (vm *vmInfoType) startQemuVm(enableNetboot, haveManagerLock bool,
	pidfile string, nCpus uint, netOptions []string,
	tapFiles []*os.File) error {
	qemuInfo, err := getQemuInfo(vm.ArchitectureType, vm.manager.Logger)
	if err != nil {
		return err
	}
	machine := []string{qemuInfo.getMachine(vm.MachineType)}
	if vm.ArchitectureType == proto.ArchitectureTypeRuntime {
		machine = append(machine, "accel=kvm")
	}
	cmd := exec.Command(qemuInfo.command,
		"-machine", strings.Join(machine, ","),
		"-cpu", qemuInfo.cpuModel,
		"-rtc", "base=utc,clock=host", // TODO(rgooch): consider if needed.
		"-nodefaults",
		"-name", vm.ipAddress,
		"-m", fmt.Sprintf("%dM", vm.MemoryInMiB),
		"-smbios", "type=1,product=SmallStack",
		"-smp", fmt.Sprintf("cpus=%d", nCpus),
		"-serial",
		"unix:"+filepath.Join(vm.dirname, serialSockFilename)+",server,nowait",
		"-qmp", "unix:"+vm.monitorSockname+",server,nowait",
		"-pidfile", pidfile,
		"-daemonize")
	// Deal with backwards-incompatible option changes.
	if qemuInfo.version.major < 8 {
		cmd.Args = append(cmd.Args,
			"-chroot", vm.getLogsDirectory(),
			"-runas", vm.manager.Username,
		)
	} else if qemuInfo.version.major < 9 {
		cmd.Args = append(cmd.Args,
			"-run-with", "chroot="+vm.getLogsDirectory(),
			"-runas", vm.manager.Username,
		)
	} else {
		cmd.Args = append(cmd.Args,
			"-run-with", "chroot="+vm.getLogsDirectory(),
			"-run-with", "user="+vm.manager.Username,
		)
	}
	switch vm.FirmwareType {
	case proto.FirmwareUEFI:
		cmd.Args = append(cmd.Args,
			"-drive",
			"if=pflash,format=raw,file="+qemuInfo.efiFlash+",readonly=on")
	}
	var interfaceDriver string
	if !vm.DisableVirtIO {
		interfaceDriver = ",if=virtio"
	}
	if debugRoot := vm.getDebugRoot(); debugRoot != "" {
		options := interfaceDriver + ",discard=off"
		cmd.Args = append(cmd.Args,
			"-drive", "file="+debugRoot+",format=raw"+options)
	} else if kernelPath := vm.getActiveKernelPath(); kernelPath != "" {
		kernelOptions := []string{"net.ifnames=0"}
		if vm.ExtraKernelOptions != "" {
			kernelOptions = append(kernelOptions, vm.ExtraKernelOptions)
		}
		kernelOptionsString := strings.Join(kernelOptions, " ")
		cmd.Args = append(cmd.Args, "-kernel", kernelPath)
		if initrdPath := vm.getActiveInitrdPath(); initrdPath != "" {
			cmd.Args = append(cmd.Args,
				"-initrd", initrdPath,
				"-append",
				util.MakeKernelOptionsWithParams(util.MakeKernelOptionsParams{
					ArchitectureType: vm.ArchitectureType,
					ExtraOptions:     kernelOptionsString,
					RootDevice:       "LABEL=" + vm.rootLabelSaved(false),
				}),
			)
		} else {
			cmd.Args = append(cmd.Args,
				"-append",
				util.MakeKernelOptionsWithParams(util.MakeKernelOptionsParams{
					ArchitectureType: vm.ArchitectureType,
					ExtraOptions:     kernelOptionsString,
					RootDevice:       "/dev/vda1",
				}),
			)
		}
	} else if enableNetboot {
		cmd.Args = append(cmd.Args, "-boot", "order=n")
	}
	cmd.Args = append(cmd.Args, netOptions...)
	if vm.manager.ShowVgaConsole {
		cmd.Args = append(cmd.Args, "-vga", "std")
	} else {
		switch vm.ConsoleType {
		case proto.ConsoleNone:
			cmd.Args = append(cmd.Args, "-nographic")
		case proto.ConsoleDummy:
			cmd.Args = append(cmd.Args, "-display", "none")
			cmd.Args = append(cmd.Args, qemuInfo.displayArgs...)
		case proto.ConsoleVNC:
			cmd.Args = append(cmd.Args,
				"-display", "vnc=unix:"+filepath.Join(vm.dirname, "vnc"))
			cmd.Args = append(cmd.Args, qemuInfo.displayArgs...)
			cmd.Args = append(cmd.Args, qemuInfo.vncArgs...)
		}
	}
	for index, volume := range vm.VolumeLocations {
		var volumeFormat proto.VolumeFormat
		var volumeInterface proto.VolumeInterface
		if index < len(vm.Volumes) {
			volumeFormat = vm.Volumes[index].Format
			volumeInterface = vm.Volumes[index].Interface
		}
		if vm.DisableVirtIO && volumeInterface == proto.VolumeInterfaceVirtIO {
			volumeInterface = proto.VolumeInterfaceIDE
		}
		// For the simple cases (VirtIO and IDE), use old-style flags to
		// maintain compatibility with old versions of QEMU (like 2.0.0).
		switch volumeInterface {
		case proto.VolumeInterfaceVirtIO, proto.VolumeInterfaceIDE:
			cmd.Args = append(cmd.Args,
				"-drive", fmt.Sprintf(
					"file=%s,format=%s,discard=off,if=%s",
					volume.Filename, volumeFormat, volumeInterface))
			continue
		case proto.VolumeInterfaceDFM:
			cmd.Args = append(cmd.Args,
				"-device", fmt.Sprintf("dfm,filename=%s", volume.Filename))
			continue
		}
		cmd.Args = append(cmd.Args,
			"-blockdev", fmt.Sprintf(
				"driver=%s,node-name=blk%d,file.driver=file,file.filename=%s",
				volumeFormat, index, volume.Filename))
		switch volumeInterface {
		case proto.VolumeInterfaceVirtIO:
			cmd.Args = append(cmd.Args,
				"-device", fmt.Sprintf(
					"virtio-blk,drive=blk%d", index))
		case proto.VolumeInterfaceIDE:
			cmd.Args = append(cmd.Args,
				"-device", fmt.Sprintf(
					"ide-hd,drive=blk%d", index))
		case proto.VolumeInterfaceNVMe:
			cmd.Args = append(cmd.Args,
				"-device", fmt.Sprintf(
					"nvme,serial=fu%s-%d,drive=blk%d",
					vm.Address.IpAddress, index, index))
		default:
			return fmt.Errorf("invalid volume interface: %v", volumeInterface)
		}
	}
	if cid, err := vm.manager.GetVmCID(vm.Address.IpAddress); err != nil {
		return err
	} else if cid > 2 {
		cmd.Args = append(cmd.Args,
			"-device",
			fmt.Sprintf("vhost-vsock-pci,id=vhost-vsock-pci0,guest-cid=%d",
				cid))
	}
	if vm.WatchdogModel != proto.WatchdogModelNone {
		cmd.Args = append(cmd.Args,
			"-watchdog-action", vm.WatchdogAction.String(),
			"-device", vm.WatchdogModel.String())
	}
	os.Remove(vm.getBootLogFilename())
	cmd.Dir = vm.getLogsDirectory()
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "VM_ARCH="+vm.ArchitectureType.String())
	cmd.Env = append(cmd.Env, "VM_HOSTNAME="+vm.Hostname)
	if len(vm.OwnerGroups) > 0 {
		cmd.Env = append(cmd.Env,
			"VM_OWNER_GROUPS="+strings.Join(vm.OwnerGroups, ","))
	}
	cmd.Env = append(cmd.Env,
		"VM_OWNER_USERS="+strings.Join(vm.OwnerUsers, ","))
	cmd.Env = append(cmd.Env, "VM_PRIMARY_IP_ADDRESS="+vm.ipAddress)
	cmd.ExtraFiles = tapFiles // Start at fd=3 for QEMU.
	if output, err := cmd.CombinedOutput(); err != nil {
		vm.logger.Printf("Failed QEMU command: %v\n", cmd.Args)
		return fmt.Errorf("error starting QEMU: %s: %s", err, output)
	} else if len(output) > 0 {
		vm.logger.Printf("QEMU started. Output: \"%s\"\n", string(output))
	} else {
		vm.logger.Println("QEMU started.")
	}
	return nil
}
