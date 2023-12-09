package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

func checkVmsSubcommand(args []string, logger log.DebugLogger) error {
	return checkVms()
}

func checkVms() error {
	dirname := filepath.Join(*stateDir, "VMs")
	vms, err := fsutil.ReadDirnames(dirname, false)
	if err != nil {
		return err
	}
	for _, vm := range vms {
		filename := filepath.Join(dirname, vm, "info.json")
		var vmInfo proto.LocalVmInfo
		if err := json.ReadFromFile(filename, &vmInfo); err != nil {
			return err
		}
		for index, volume := range vmInfo.VolumeLocations {
			var statbuf wsyscall.Stat_t
			if err := wsyscall.Stat(volume.Filename, &statbuf); err != nil {
				return err
			}
			if vmInfo.Volumes[index].Size != uint64(statbuf.Size) {
				fmt.Fprintf(os.Stderr, "%s size: %s should be: %s\n",
					volume.Filename,
					format.FormatBytes(uint64(statbuf.Size)),
					format.FormatBytes(vmInfo.Volumes[index].Size))
				continue
			}
			requiredBlocks := vmInfo.Volumes[index].Size >> 9
			if requiredBlocks<<9 < vmInfo.Volumes[index].Size {
				requiredBlocks++
			}
			if uint64(statbuf.Blocks) < requiredBlocks {
				shift, unit := format.GetMiltiplier(uint64(statbuf.Blocks) << 9)
				shiftRequired, unitRequired := format.GetMiltiplier(
					requiredBlocks << 9)
				if shiftRequired < shift {
					shift = shiftRequired
					unit = unitRequired
				}
				fmt.Fprintf(os.Stderr, "%s alloc: %d %sB should be: %d %sB\n",
					volume.Filename,
					(statbuf.Blocks<<9)>>shift, unit,
					(requiredBlocks<<9)>>shift, unit)
			}
		}
	}
	return nil
}
