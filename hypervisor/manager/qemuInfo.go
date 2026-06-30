package manager

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/Cloud-Foundations/Dominator/lib/log"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

const (
	qemuVersionMatchString = "QEMU emulator version"
)

type qemuInfoType struct {
	autoMachine   string
	command       string
	cpuModel      string
	cpuModelFlags map[string]struct{}
	displayArgs   []string
	efiFlash      string
	version       qemuVersionType
	vncArgs       []string
}

type qemuVersionType struct {
	major    uint
	minor    uint
	subminor uint
}

var (
	qemuToAutoMachine = map[string]string{
		"qemu-system-x86_64":  "pc",
		"qemu-system-aarch64": "virt",
	}
	qemuToCpuModel = map[string]string{
		"qemu-system-x86_64":  "host", // Take full advantage of host CPU.
		"qemu-system-aarch64": "max",
	}
	qemuToDisplayArgs = map[string][]string{
		"qemu-system-x86_64":  {"-vga", "std"},
		"qemu-system-aarch64": {"-device", "virtio-gpu-pci"},
	}
	qemuToEfiFlash = map[string]string{
		"qemu-system-x86_64":  "/usr/share/ovmf/OVMF.fd",
		"qemu-system-aarch64": "/usr/share/AAVMF/AAVMF_CODE.fd",
	}
	qemuToVncArgs = map[string][]string{
		"qemu-system-x86_64": {"-usb", "-device", "usb-tablet"},
		"qemu-system-aarch64": {
			"-device", "virtio-keyboard",
			"-device", "virtio-tablet-device"},
	}
	qemuWrapperCommand = flag.String("qemuCommand", "", "QEMU command")

	qemuInfoMutex sync.Mutex
	qemuInfos     = make(map[string]qemuInfoType) // Key: QEMU command.
)

func getQemuInfo(architectureType proto.ArchitectureType,
	logger log.Logger) (qemuInfoType, error) {
	var qemuCommand string
	switch architectureType {
	case proto.ArchitectureTypeAmd64:
		qemuCommand = "qemu-system-x86_64"
	case proto.ArchitectureTypeArm64:
		qemuCommand = "qemu-system-aarch64"
	default:
		return qemuInfoType{},
			fmt.Errorf("unsupported architecture type: %d", architectureType)
	}
	qemuInfoMutex.Lock()
	if qemuInfo, ok := qemuInfos[qemuCommand]; ok {
		qemuInfoMutex.Unlock()
		return qemuInfo, nil
	}
	qemuInfoMutex.Unlock()
	// Get autoMachine.
	autoMachine, ok := qemuToAutoMachine[qemuCommand]
	if !ok {
		return qemuInfoType{}, fmt.Errorf("no autoMachine for: %s", qemuCommand)
	}
	// Get CPU model.
	cpuModel, ok := qemuToCpuModel[qemuCommand]
	if !ok {
		return qemuInfoType{}, fmt.Errorf("no CPU model for: %s", qemuCommand)
	}
	cpuModelFlags, err := getQemuCpuModelFlags(qemuCommand)
	if err != nil {
		return qemuInfoType{}, err
	}
	if _, ok := cpuModelFlags["invtsc"]; ok {
		cpuModel += ",+invtsc,migratable=no" // Try hard to provide TSC.
	} else if _, ok := cpuModelFlags["kvmclock"]; ok {
		cpuModel += ",+kvmclock" // Fall back to something faster than HPET.
	}
	// Get display arguments.
	displayArgs, ok := qemuToDisplayArgs[qemuCommand]
	if !ok {
		return qemuInfoType{},
			fmt.Errorf("no display args for: %s", qemuCommand)
	}
	// Get EFI flash filename.
	efiFlash, ok := qemuToEfiFlash[qemuCommand]
	if !ok {
		return qemuInfoType{}, fmt.Errorf("no EFI flash for: %s", qemuCommand)
	}
	// Get QEMU version.
	qemuVersion, err := getQemuVersion(qemuCommand, logger)
	if err != nil {
		return qemuInfoType{}, err
	}
	// Get VNC arguments.
	vncArgs, ok := qemuToVncArgs[qemuCommand]
	if !ok {
		return qemuInfoType{}, fmt.Errorf("no VNC args for: %s", qemuCommand)
	}
	qemuInfo := qemuInfoType{
		autoMachine:   autoMachine,
		command:       qemuCommand,
		cpuModel:      cpuModel,
		cpuModelFlags: cpuModelFlags,
		displayArgs:   displayArgs,
		efiFlash:      efiFlash,
		version:       qemuVersion,
		vncArgs:       vncArgs,
	}
	if *qemuWrapperCommand != "" {
		qemuInfo.command = *qemuWrapperCommand
	}
	qemuInfoMutex.Lock()
	if qemuInfo, ok := qemuInfos[qemuCommand]; ok {
		qemuInfoMutex.Unlock()
		return qemuInfo, nil
	}
	qemuInfos[qemuCommand] = qemuInfo
	qemuInfoMutex.Unlock()
	logger.Printf("Detected %s version: %d.%d.%d\n",
		qemuCommand, qemuVersion.major, qemuVersion.minor, qemuVersion.subminor)
	return qemuInfo, nil
}

func getQemuCpuModelFlags(qemuCommand string) (map[string]struct{}, error) {
	cmd := exec.Command(qemuCommand, "-cpu", "help")
	cmd.Dir = "/tmp"
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	var modelFlags map[string]struct{} // nil means header not yet found.
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		if modelFlags == nil {
			if line == "Recognized CPUID flags:" {
				modelFlags = make(map[string]struct{})
			}
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			break
		}
		for _, field := range strings.Fields(line[2:]) {
			modelFlags[field] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return modelFlags, nil
}

func getQemuVersion(qemuCommand string,
	logger log.Logger) (qemuVersionType, error) {
	cmd := exec.Command(qemuCommand, "-version")
	cmd.Dir = "/tmp"
	stdout, err := cmd.Output()
	if err != nil {
		return qemuVersionType{}, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		if !strings.HasPrefix(line, qemuVersionMatchString) {
			continue
		}
		var info qemuVersionType
		_, err := fmt.Sscanf(line[len(qemuVersionMatchString):], " %d.%d.%d ",
			&info.major, &info.minor, &info.subminor)
		if err != nil {
			return qemuVersionType{}, err
		}
		return info, nil
	}
	if err := scanner.Err(); err != nil {
		return qemuVersionType{}, err
	}
	return qemuVersionType{}, fmt.Errorf("no QEMU version info found")
}

func (qi *qemuInfoType) getMachine(machineType proto.MachineType) string {
	if machineType == proto.MachineTypeAuto {
		return qi.autoMachine
	}
	return machineType.String()
}
