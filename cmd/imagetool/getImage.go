package main

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/nulllogger"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

func getImageSubcommand(args []string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	err := getImageAndWrite(objectClient, args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("Error getting image: %s", err)
	}
	return nil
}

func getImageAndWrite(objectClient *objectclient.ObjectClient, name,
	dirname string, logger log.DebugLogger) error {
	startTime := time.Now()
	fs, objectsGetter, err := getImageForUnpack(objectClient, name)
	if err != nil {
		return err
	}
	logger.Debugf(0, "Got image: %s in %s\n",
		name, format.Duration(time.Since(startTime)))
	startTime = time.Now()
	err = util.Unpack(fs, objectsGetter, dirname, nulllogger.New())
	if err != nil {
		return err
	}
	duration := time.Since(startTime)
	speed := uint64(float64(fs.TotalDataBytes) / duration.Seconds())
	logger.Debugf(0, "Downloaded and unpacked %d objects (%s) in %s (%s/s)\n",
		fs.NumRegularInodes, format.FormatBytes(fs.TotalDataBytes),
		format.Duration(duration), format.FormatBytes(speed))
	return util.WriteImageName(dirname, name)
}

func getImageForUnpack(objectClient *objectclient.ObjectClient, name string) (
	*filesystem.FileSystem, objectserver.ObjectsGetter, error) {
	fs, err := getTypedImage(name)
	if err != nil {
		return nil, nil, err
	}
	if *computedFilesRoot == "" {
		return fs, objectClient, nil
	}
	objectsGetter, err := util.ReplaceComputedFiles(fs,
		&util.ComputedFilesData{RootDirectory: *computedFilesRoot},
		objectClient)
	if err != nil {
		return nil, nil, err
	}
	return fs, objectsGetter, nil
}
