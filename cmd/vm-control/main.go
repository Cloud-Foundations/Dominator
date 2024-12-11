package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/flags/commands"
	"github.com/Cloud-Foundations/Dominator/lib/flags/loadflags"
	"github.com/Cloud-Foundations/Dominator/lib/flagutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/cmdlogger"
	"github.com/Cloud-Foundations/Dominator/lib/net/rrdialer"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/srpc/setupclient"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
	hyper_proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

var (
	adjacentVM = flag.String("adjacentVM", "",
		"IP address of VM adjacent (same Hypervisor) to VM being created")
	consoleType hyper_proto.ConsoleType
	cpuPriority = flag.Int("cpuPriority", 0,
		"CPU priority (-20:+19) for VM process on Hypervisor")
	destroyOnPowerdown = flag.Bool("destroyOnPowerdown", false,
		"If true, destroy VM if it powers down internally")
	destroyProtection = flag.Bool("destroyProtection", false,
		"If true, do not destroy running VM")
	disableVirtIO = flag.Bool("disableVirtIO", false,
		"If true, disable virtio drivers, reducing I/O performance")
	dhcpTimeout = flag.Duration("dhcpTimeout", time.Minute,
		"Time to wait before timing out on DHCP request from VM")
	doNotStart = flag.Bool("doNotStart", false,
		"If true, do not start VM when creating")
	enableNetboot = flag.Bool("enableNetboot", false,
		"If true, enable boot from network for first boot")
	extraKernelOptions = flag.String("extraKernelOptions", "",
		"Extra options to pass to kernel")
	fleetManagerHostname = flag.String("fleetManagerHostname", "",
		"Hostname of Fleet Manager")
	fleetManagerPortNum = flag.Uint("fleetManagerPortNum",
		constants.FleetManagerPortNumber,
		"Port number of Fleet Resource Manager")
	forceIfNotStopped = flag.Bool("forceIfNotStopped", false,
		"If true, snapshot or restore VM even if not stopped")
	hypervisorHostname = flag.String("hypervisorHostname", "",
		"Hostname of hypervisor")
	hypervisorPortNum = flag.Uint("hypervisorPortNum",
		constants.HypervisorPortNumber, "Port number of hypervisor")
	hypervisorTagsToMatch tags.MatchTags
	identityCertFile      = flag.String("identityCertFile", "",
		"Filename of PEM-encoded cetificate availabe from metadata service (deprecated: use identityName instead)")
	identityKeyFile = flag.String("identityKeyFile", "",
		"Filename of PEM-encoded key available from metadata service (deprecated: use identityName instead)")
	identityName = flag.String("identityName", "",
		"Identity name for requesting role certificates from IdentityProvider")
	includeUnhealthy = flag.Bool("includeUnhealthy", false,
		"If true, list connected but unhealthy hypervisors")
	imageFile = flag.String("imageFile", "",
		"Name of RAW image file to boot with")
	imageName = flag.String("imageName", "",
		"Name of image to boot with")
	imageServerHostname = flag.String("imageServerHostname", "",
		"Hostname of image server")
	imageServerPortNum = flag.Uint("imageServerPortNum",
		constants.ImageServerPortNumber,
		"Port number of image server")
	imageTimeout = flag.Duration("imageTimeout", time.Minute,
		"Time to wait before timing out on image fetch")
	imageURL = flag.String("imageURL", "",
		"Name of URL of image to boot with")
	initialiseSecondaryVolumes = flag.Bool("initialiseSecondaryVolumes", false,
		"If true, initialise secondary volumes")
	localVmCreate = flag.String("localVmCreate", "",
		"Command to make local VM when exporting. The VM name is given as the argument. The VM JSON is available on stdin")
	localVmDestroy = flag.String("localVmDestroy", "",
		"Command to destroy local VM when exporting. The VM name is given as the argument")
	location = flag.String("location", "",
		"Location to search for hypervisors")
	machineType      hyper_proto.MachineType
	memory           flagutil.Size
	milliCPUs        = flag.Uint("milliCPUs", 0, "milli CPUs (default 250)")
	placement        placementType
	placementCommand = flag.String("placementCommand", "",
		"Command to make placement decisions when creating/copying/moving VM")
	minFreeBytes     = flagutil.Size(256 << 20)
	overlayDirectory = flag.String("overlayDirectory", "",
		"Directory tree of files to overlay on top of the image")
	overlayPrefix = flag.String("overlayPrefix", "/",
		"Prefix to add to overlay filenames")
	ownerGroups      flagutil.StringList
	ownerUsers       flagutil.StringList
	patchLogFilename = flag.String("patchLogFilename", "",
		"Name file to write VM patch log to")
	probePortNum = flag.Uint("probePortNum", 0, "Port number on VM to probe")
	probeTimeout = flag.Duration("probeTimeout", time.Minute*5,
		"Time to wait before timing out on probing VM port")
	secondarySubnetIDs         flagutil.StringList
	secondaryVolumeSizes       flagutil.SizeList
	secondaryVolumesInitParams = flag.String("secondaryVolumesInitParams", "",
		"File containing initialisation parameters for secondary volumes")
	serialPort = flag.Uint("serialPort", 0,
		"Serial port number on VM")
	skipBackup = flag.Bool("skipBackup", false,
		"If true, do not make a backup when patching/replacing the VM image")
	skipBootloader = flag.Bool("skipBootloader", false,
		"If true, directly boot into the kernel")
	skipMemoryCheck = flag.Bool("skipMemoryCheck", false,
		"If true, skip memory availability check before creating VM")
	spreadVolumes = flag.Bool("spreadVolumes", false,
		"If true, spread the VM volumes across backing stores")
	storageIndices flagutil.UintList
	subnetId       = flag.String("subnetId", "",
		"Subnet ID to launch VM in")
	requestIPs   flagutil.StringList
	roundupPower = flag.Uint64("roundupPower", 28,
		"power of 2 to round up root volume size")
	scanFilename = flag.String("scanFilename", "",
		"Name of file to write scanned VM root to")
	snapshotName     = flag.String("snapshotName", "", "Optional snapshot name")
	snapshotRootOnly = flag.Bool("snapshotRootOnly", false,
		"If true, snapshot only the root volume")
	traceMetadata = flag.Bool("traceMetadata", false,
		"If true, trace metadata calls until interrupted")
	userDataFile = flag.String("userDataFile", "",
		"Name file containing user-data accessible from the metadata server")
	virtualCPUs = flag.Uint("vCPUs", 0,
		"virtual CPUs (default rounds up milliCPUs)")
	vmHostname    = flag.String("vmHostname", "", "Hostname for VM")
	vmTags        tags.Tags
	vmTagsToMatch tags.MatchTags
	vncViewer     = flag.String("vncViewer", defaultVncViewer,
		"Path to VNC viewer")
	volumeFilename = flag.String("volumeFilename", "",
		"Name of file to write volume data to")
	volumeFormat hyper_proto.VolumeFormat
	volumeIndex  = flag.Uint("volumeIndex", 0,
		"Index of volume to get or delete")
	volumeIndices    flagutil.UintList
	volumeInterfaces volumeInterfaceList
	volumeSize       flagutil.Size
	volumeTypes      volumeTypeList
	watchdogAction   hyper_proto.WatchdogAction
	watchdogModel    hyper_proto.WatchdogModel

	logger   log.DebugLogger
	rrDialer *rrdialer.Dialer
)

func init() {
	flag.Var(&consoleType, "consoleType",
		"type of graphical console (default none)")
	flag.Var(&hypervisorTagsToMatch, "hypervisorTagsToMatch",
		"Tags to match when getting/listing or creating/copying/moving VMs")
	flag.Var(&machineType, "machineType",
		"Type of machine to emulate (default generic PC)")
	flag.Var(&memory, "memory", "memory (default 1GiB)")
	flag.Var(&minFreeBytes, "minFreeBytes",
		"minimum number of free bytes in root volume")
	flag.Var(&placement, "placement",
		"Placement choice when selecting Hypervisor to create/copy/move VM")
	flag.Var(&ownerGroups, "ownerGroups", "Groups who own the VM")
	flag.Var(&ownerUsers, "ownerUsers", "Extra users who own the VM")
	flag.Var(&requestIPs, "requestIPs", "Request specific IPs, if available")
	flag.Var(&secondarySubnetIDs, "secondarySubnetIDs", "Secondary Subnet IDs")
	flag.Var(&secondaryVolumeSizes, "secondaryVolumeSizes",
		"Sizes for secondary volumes")
	flag.Var(&storageIndices, "storageIndices",
		"Indices for volume backing stores")
	flag.Var(&vmTags, "vmTags", "Tags to apply to VM")
	flag.Var(&vmTagsToMatch, "vmTagsToMatch", "Tags to match when listing")
	flag.Var(&volumeFormat, "volumeFormat",
		"Format of image provided by file or URL (default raw)")
	flag.Var(&volumeIndices, "volumeIndices", "Index of volumes")
	flag.Var(&volumeInterfaces, "volumeInterfaces",
		"Interfaces (device type presented to VM) for volumes (default virtio)")
	flag.Var(&volumeSize, "volumeSize", "New size of specified volume")
	flag.Var(&volumeTypes, "volumeTypes",
		"Types for volumes (default persistent)")
	flag.Var(&watchdogAction, "watchdogAction",
		"Action to take when watchdog times out (default none)")
	flag.Var(&watchdogModel, "watchdogModel",
		"Model of virtual hardware watchdog (default none)")
}

func printUsage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "Usage: vm-control [flags...] command [args...]")
	fmt.Fprintln(w, "Common flags:")
	flag.PrintDefaults()
	fmt.Fprintln(w, "Commands:")
	commands.PrintCommands(w, subcommands)
}

var subcommands = []commands.Command{
	{"add-vm-volumes", "IPaddr", 1, 1, addVmVolumesSubcommand},
	{"become-primary-vm-owner", "IPaddr", 1, 1, becomePrimaryVmOwnerSubcommand},
	{"change-vm-console-type", "IPaddr", 1, 1, changeVmConsoleTypeSubcommand},
	{"change-vm-cpu-priority", "IPaddr", 1, 1, changeVmCpuPrioritySubcommand},
	{"change-vm-cpus", "IPaddr", 1, 1, changeVmCPUsSubcommand},
	{"change-vm-destroy-protection", "IPaddr", 1, 1,
		changeVmDestroyProtectionSubcommand},
	{"change-vm-machine-type", "IPaddr", 1, 1, changeVmMachineTypeSubcommand},
	{"change-vm-memory", "IPaddr", 1, 1, changeVmMemorySubcommand},
	{"change-vm-owner-groups", "IPaddr", 1, 1, changeVmOwnerGroupsSubcommand},
	{"change-vm-owner-users", "IPaddr", 1, 1, changeVmOwnerUsersSubcommand},
	{"change-vm-tags", "IPaddr", 1, 1, changeVmTagsSubcommand},
	{"change-vm-vcpus", "IPaddr", 1, 1, changeVmVirtualCPUsSubcommand},
	{"change-vm-volume-interfaces", "IPaddr", 1, 1,
		changeVmVolumeInterfacesSubcommand},
	{"change-vm-volume-size", "IPaddr", 1, 1, changeVmVolumeSizeSubcommand},
	{"connect-to-vm-console", "IPaddr", 1, 1, connectToVmConsoleSubcommand},
	{"connect-to-vm-serial-port", "IPaddr", 1, 1,
		connectToVmSerialPortSubcommand},
	{"copy-vm", "IPaddr", 1, 1, copyVmSubcommand},
	{"create-vm", "", 0, 0, createVmSubcommand},
	{"debug-vm-image", "IPaddr", 1, 1, debugVmImageSubcommand},
	{"delete-vm-volume", "IPaddr", 1, 1, deleteVmVolumeSubcommand},
	{"destroy-vm", "IPaddr", 1, 1, destroyVmSubcommand},
	{"discard-vm-old-image", "IPaddr", 1, 1, discardVmOldImageSubcommand},
	{"discard-vm-old-user-data", "IPaddr", 1, 1,
		discardVmOldUserDataSubcommand},
	{"discard-vm-snapshot", "IPaddr", 1, 1, discardVmSnapshotSubcommand},
	{"export-local-vm", "IPaddr", 1, 1, exportLocalVmSubcommand},
	{"export-virsh-vm", "IPaddr", 1, 1, exportVirshVmSubcommand},
	{"get-hypervisors", "", 0, 0, getHypervisorsSubcommand},
	{"get-vm-info", "IPaddr", 1, 1, getVmInfoSubcommand},
	{"get-vm-infos", "", 0, 0, getVmInfosSubcommand},
	{"get-vm-user-data", "IPaddr", 1, 1, getVmUserDataSubcommand},
	{"get-vm-volume", "IPaddr", 1, 1, getVmVolumeSubcommand},
	{"import-local-vm", "info-file root-volume", 2, 2, importLocalVmSubcommand},
	{"import-virsh-vm", "MACaddr domain [[MAC IP]...]", 2, -1,
		importVirshVmSubcommand},
	{"list-hypervisors", "", 0, 0, listHypervisorsSubcommand},
	{"list-locations", "[TopLocation]", 0, 1, listLocationsSubcommand},
	{"list-vms", "", 0, 0, listVMsSubcommand},
	{"migrate-vm", "IPaddr", 1, 1, migrateVmSubcommand},
	{"parse-virsh-xml", "filename", 1, 1, parseVirshXmlSubcommand},
	{"patch-vm-image", "IPaddr", 1, 1, patchVmImageSubcommand},
	{"probe-vm-port", "IPaddr", 1, 1, probeVmPortSubcommand},
	{"reboot-vm", "IPaddr", 1, 1, rebootVmSubcommand},
	{"replace-vm-credentials", "IPaddr (deprecated)", 1, 1,
		replaceVmCredentialsSubcommand},
	{"replace-vm-identity", "IPaddr", 1, 1, replaceVmIdentitySubcommand},
	{"replace-vm-image", "IPaddr", 1, 1, replaceVmImageSubcommand},
	{"replace-vm-user-data", "IPaddr", 1, 1, replaceVmUserDataSubcommand},
	{"restore-vm", "source", 1, 1, restoreVmSubcommand},
	{"restore-vm-from-snapshot", "IPaddr", 1, 1,
		restoreVmFromSnapshotSubcommand},
	{"restore-vm-image", "IPaddr", 1, 1, restoreVmImageSubcommand},
	{"restore-vm-user-data", "IPaddr", 1, 1, restoreVmUserDataSubcommand},
	{"reorder-vm-volumes", "IPaddr", 1, 1, reorderVmVolumesSubcommand},
	{"set-vm-migrating", "IPaddr", 1, 1, setVmMigratingSubcommand},
	{"snapshot-vm", "IPaddr", 1, 1, snapshotVmSubcommand},
	{"save-vm", "IPaddr destination", 2, 2, saveVmSubcommand},
	{"scan-vm-root", "IPaddr", 1, 1, scanVmRootSubcommand},
	{"start-vm", "IPaddr", 1, 1, startVmSubcommand},
	{"stop-vm", "IPaddr", 1, 1, stopVmSubcommand},
	{"trace-vm-metadata", "IPaddr", 1, 1, traceVmMetadataSubcommand},
	{"unset-vm-migrating", "IPaddr", 1, 1, unsetVmMigratingSubcommand},
}

func doMain() int {
	if err := loadflags.LoadForCli("vm-control"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	flag.Usage = printUsage
	flag.Parse()
	if flag.NArg() < 1 {
		printUsage()
		return 2
	}
	// Sanity check on flags.
	if *cpuPriority < -20 || *cpuPriority > 19 {
		fmt.Fprintf(os.Stderr, "invalid cpuPriority: %d\n", *cpuPriority)
		return 3
	}
	if memory > 0 && memory < 1<<20 {
		fmt.Fprintf(os.Stderr, "unreasonably small memory: %s\n",
			memory.String())
		return 3
	}
	logger = cmdlogger.New()
	srpc.SetDefaultLogger(logger)
	if err := setupclient.SetupTls(false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var err error
	rrDialer, err = rrdialer.New(&net.Dialer{Timeout: time.Second * 10}, "",
		logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer rrDialer.WaitForBackgroundResults(time.Second)
	return commands.RunCommands(subcommands, printUsage, logger)
}

func main() {
	os.Exit(doMain())
}
