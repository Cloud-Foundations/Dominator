package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
)

func makeRawImageSubcommand(args []string, logger log.DebugLogger) error {
	objectsGetter := getObjectsGetter(logger)
	if err := makeRawImage(objectsGetter, args[0], args[1]); err != nil {
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
	rawFilename string) error {
	if os.Geteuid() != 0 {
		return reExecAsRoot()
	}
	fs, objectsGetter, imageName, err := getImageForUnpack(objectsGetter, name)
	if err != nil {
		return err
	}
	options := util.WriteRawOptions{
		AllocateBlocks:    *allocateBlocks,
		InitialImageName:  imageName,
		InstallBootloader: *makeBootable,
		MinimumFreeBytes:  uint64(minFreeBytes),
		WriteFstab:        *makeBootable,
		RootLabel:         *rootLabel,
		RoundupPower:      *roundupPower,
	}
	if overlayFiles, err := loadOverlayFiles(); err != nil {
		return err
	} else {
		options.OverlayFiles = overlayFiles
	}
	return util.WriteRawWithOptions(fs, objectsGetter, rawFilename,
		fsutil.PublicFilePerms, tableType, options, logger)
}
