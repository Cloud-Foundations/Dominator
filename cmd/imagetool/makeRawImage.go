package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/json"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/types"
	"github.com/Cloud-Foundations/Dominator/proto/installer"
)

func makeRawImageSubcommand(args []string, logger log.DebugLogger) error {
	objectsGetter := getObjectsGetter(logger)
	err := makeRawImage(objectsGetter, args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error making raw image: %s", err)
	}
	return nil
}

func loadOverlayFiles() (map[string][]byte, error) {
	if *overlayDirectory == "" {
		return nil, nil
	}
	overlayFiles := make(map[string][]byte)
	err := filepath.Walk(*overlayDirectory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			overlayFiles[path[len(*overlayDirectory):]] = data
			return nil
		})
	return overlayFiles, err
}

func makeRawImage(objectsGetter objectserver.ObjectsGetter, name string,
	rawFilename string, logger log.DebugLogger) error {
	if os.Geteuid() != 0 {
		return reExecAsRoot()
	}
	gid, uid := getSudoIDs()
	fs, objectsGetter, imageName, err := getImageForUnpack(objectsGetter, name)
	if err != nil {
		return err
	}
	var extraPartitions []installer.Partition
	if *extraPartitionsFilename != "" {
		err := json.ReadFromFile(*extraPartitionsFilename, &extraPartitions)
		if err != nil {
			return fmt.Errorf("error reading extra partitions data: %s", err)
		}
	}
	options := util.WriteRawOptions{
		AllocateBlocks:     *allocateBlocks,
		ExtraKernelOptions: *extraKernelOptions,
		ExtraPartitions:    extraPartitions,
		InitialImageName:   imageName,
		InstallBootloader:  *makeBootable,
		MinimumBytes:       types.Bytes(minBytes),
		MinimumFreeBytes:   uint64(minFreeBytes),
		WriteFstab:         *makeBootable,
		RootLabel:          *rootLabel,
		RoundupPower:       *roundupPower,
	}
	if overlayFiles, err := loadOverlayFiles(); err != nil {
		return err
	} else {
		options.OverlayFiles = overlayFiles
	}
	if len(extraPartitions) > 0 {
		if options.OverlayFiles == nil {
			options.OverlayFiles = make(map[string][]byte)
		}
		secondaryFstab := &bytes.Buffer{}
		for _, partition := range extraPartitions {
			util.WriteFstabEntry(secondaryFstab,
				"LABEL="+partition.FileSystemLabel,
				partition.MountPoint,
				partition.FileSystemType.String(),
				"", 0, 2)
		}
		secondaryFstab.Write(options.OverlayFiles["/etc/fstab"])
		options.OverlayFiles["/etc/fstab"] = secondaryFstab.Bytes()
	}
	err = util.WriteRawWithOptions(fs, objectsGetter, rawFilename,
		fsutil.PublicFilePerms, tableType, options, logger)
	if err != nil {
		return err
	}
	if gid != 0 && uid != 0 {
		if err := os.Chown(rawFilename, int(uid), int(gid)); err != nil {
			logger.Println(err)
		}
	}
	return nil
}
