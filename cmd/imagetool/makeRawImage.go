package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

const filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP |
	syscall.S_IROTH

func makeRawImageSubcommand(args []string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	if err := makeRawImage(objectClient, args[0], args[1]); err != nil {
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

func makeRawImage(objectClient *objectclient.ObjectClient, name,
	rawFilename string) error {
	fs, objectsGetter, err := getImageForUnpack(objectClient, name)
	if err != nil {
		return err
	}
	options := util.WriteRawOptions{
		AllocateBlocks:    *allocateBlocks,
		InitialImageName:  name,
		InstallBootloader: *makeBootable,
		MinimumFreeBytes:  *minFreeBytes,
		WriteFstab:        *makeBootable,
		RootLabel:         *rootLabel,
		RoundupPower:      *roundupPower,
	}
	if overlayFiles, err := loadOverlayFiles(); err != nil {
		return err
	} else {
		options.OverlayFiles = overlayFiles
	}
	return util.WriteRawWithOptions(fs, objectsGetter, rawFilename, filePerms,
		tableType, options, logger)
}
