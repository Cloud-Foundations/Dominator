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
	cmd := exec.Command(*qemuCommand,
		"-machine", fmt.Sprintf("%s,accel=kvm", vm.MachineType),
		"-cpu", "host", // Allow the VM to take full advantage of host CPU.
		"-nodefaults",
		"-name", vm.ipAddress,
		"-m", fmt.Sprintf("%dM", vm.MemoryInMiB),
		"-smbios", "type=1,product=SmallStack",
		"-smp", fmt.Sprintf("cpus=%d", nCpus),
		"-serial",
		"unix:"+filepath.Join(vm.dirname, serialSockFilename)+",server,nowait",
		"-chroot", "/tmp",
		"-runas", vm.manager.Username,
		"-qmp", "unix:"+vm.monitorSockname+",server,nowait",
		"-pidfile", pidfile,
		"-daemonize")
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
				util.MakeKernelOptions("LABEL="+vm.rootLabelSaved(false),
					kernelOptionsString),
			)
		} else {
			cmd.Args = append(cmd.Args,
				"-append", util.MakeKernelOptions("/dev/vda1",
					kernelOptionsString),
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
			cmd.Args = append(cmd.Args, "-display", "none", "-vga", "std")
		case proto.ConsoleVNC:
			cmd.Args = append(cmd.Args,
				"-display", "vnc=unix:"+filepath.Join(vm.dirname, "vnc"),
				"-vga", "std",
				"-usb", "-device", "usb-tablet",
			)
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
			"-watchdog", vm.WatchdogModel.String())
	}
	os.Remove(filepath.Join(vm.dirname, "bootlog"))
	cmd.Env = os.Environ()
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
		return fmt.Errorf("error starting QEMU: %s: %s", err, output)
	} else if len(output) > 0 {
		vm.logger.Printf("QEMU started. Output: \"%s\"\n", string(output))
	} else {
		vm.logger.Println("QEMU started.")
	}
	return nil
}
