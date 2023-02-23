// +build linux

package main

import (
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/slavedriver"
	"github.com/Cloud-Foundations/Dominator/lib/slavedriver/smallstack"
	"github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

type slaveDriverConfiguration struct {
	HypervisorAddress  string
	MaximumIdleSlaves  uint
	MinimumIdleSlaves  uint
	ImageIdentifier    string
	MemoryInMiB        uint64
	MilliCPUs          uint
	PreferMemoryVolume bool
	OverlayDirectory   string
	VirtualCPUs        uint
}

func createSlaveDriver(logger log.DebugLogger) (
	*slavedriver.SlaveDriver, error) {
	if *slaveDriverConfigurationFile == "" {
		return nil, nil
	}
	var configuration slaveDriverConfiguration
	err := json.ReadFromFile(*slaveDriverConfigurationFile, &configuration)
	if err != nil {
		return nil, err
	}
	createVmRequest := hypervisor.CreateVmRequest{
		DhcpTimeout:      time.Minute,
		MinimumFreeBytes: 256 << 20,
		SkipBootloader:   true,
		VmInfo: hypervisor.VmInfo{
			ImageName:   configuration.ImageIdentifier,
			MemoryInMiB: configuration.MemoryInMiB,
			MilliCPUs:   configuration.MilliCPUs,
			VirtualCPUs: configuration.VirtualCPUs,
		},
	}
	if configuration.PreferMemoryVolume {
		createVmRequest.VmInfo.Volumes = []hypervisor.Volume{
			{Type: hypervisor.VolumeTypeMemory},
		}
	}
	if configuration.OverlayDirectory != "" {
		overlayFiles, err := fsutil.ReadFileTree(configuration.OverlayDirectory,
			"/")
		if err != nil {
			return nil, err
		}
		createVmRequest.OverlayFiles = overlayFiles
	}
	slaveTrader, err := smallstack.NewSlaveTraderWithOptions(
		smallstack.SlaveTraderOptions{
			CreateRequest:     createVmRequest,
			HypervisorAddress: configuration.HypervisorAddress,
		},
		logger)
	if err != nil {
		return nil, err
	}
	slaveDriver, err := slavedriver.NewSlaveDriver(
		slavedriver.SlaveDriverOptions{
			DatabaseFilename:  filepath.Join(*stateDir, "build-slaves.json"),
			MaximumIdleSlaves: configuration.MaximumIdleSlaves,
			MinimumIdleSlaves: configuration.MinimumIdleSlaves,
			PortNumber:        *portNum,
			Purpose:           "building",
		},
		slaveTrader, logger)
	if err != nil {
		return nil, err
	}
	return slaveDriver, nil
}
