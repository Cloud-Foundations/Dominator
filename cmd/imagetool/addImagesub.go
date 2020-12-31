package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	subclient "github.com/Cloud-Foundations/Dominator/sub/client"
)

func addImagesubSubcommand(args []string, logger log.DebugLogger) error {
	imageSClient, objectClient := getClients()
	err := addImagesub(imageSClient, objectClient, args[0], args[1], args[2],
		args[3], logger)
	if err != nil {
		return fmt.Errorf("error adding image: \"%s\"\t%s", args[0], err)
	}
	return nil
}

func addImagesub(imageSClient *srpc.Client,
	objectClient *objectclient.ObjectClient,
	name, subName, filterFilename, triggersFilename string,
	logger log.DebugLogger) error {
	imageExists, err := client.CheckImage(imageSClient, name)
	if err != nil {
		return errors.New("error checking for image existence: " + err.Error())
	}
	if imageExists {
		return errors.New("image exists")
	}
	newImage := new(image.Image)
	if err := loadImageFiles(newImage, objectClient, filterFilename,
		triggersFilename); err != nil {
		return err
	}
	fs, err := pollImage(subName)
	if err != nil {
		return err
	}
	if fs, err = applyDeleteFilter(fs); err != nil {
		return err
	}
	fs = fs.Filter(newImage.Filter)
	if err := spliceComputedFiles(fs); err != nil {
		return err
	}
	if err := copyMissingObjects(fs, imageSClient, objectClient,
		subName); err != nil {
		return err
	}
	newImage.FileSystem = fs
	return addImage(imageSClient, name, newImage, logger)
}

func applyDeleteFilter(fs *filesystem.FileSystem) (
	*filesystem.FileSystem, error) {
	if *deleteFilter == "" {
		return fs, nil
	}
	filter, err := filter.Load(*deleteFilter)
	if err != nil {
		return nil, err
	}
	return fs.Filter(filter), nil
}

func copyMissingObjects(fs *filesystem.FileSystem, imageSClient *srpc.Client,
	objectClient *objectclient.ObjectClient, subName string) error {
	// Check to see which objects are in the objectserver.
	hashes := make([]hash.Hash, 0, fs.NumRegularInodes)
	for hash := range fs.HashToInodesTable() {
		hashes = append(hashes, hash)
	}
	objectSizes, err := objectClient.CheckObjects(hashes)
	if err != nil {
		return err
	}
	missingHashes := make(map[hash.Hash]struct{})
	for index, size := range objectSizes {
		if size < 1 {
			missingHashes[hashes[index]] = struct{}{}
		}
	}
	if len(missingHashes) < 1 {
		return nil
	}
	// Get missing objects from sub.
	filesForMissingObjects := make([]string, 0, len(missingHashes))
	hashToFilename := make(map[hash.Hash]string)
	for hashVal := range missingHashes {
		if inums, ok := fs.HashToInodesTable()[hashVal]; !ok {
			return fmt.Errorf("no inode for object: %x", hashVal)
		} else if files, ok := fs.InodeToFilenamesTable()[inums[0]]; !ok {
			return fmt.Errorf("no file for inode: %d", inums[0])
		} else {
			filesForMissingObjects = append(filesForMissingObjects, files[0])
			hashToFilename[hashVal] = files[0]
		}
	}
	objAdderQueue, err := objectclient.NewObjectAdderQueue(imageSClient)
	if err != nil {
		return err
	}
	subClient, err := srpc.DialHTTP("tcp",
		fmt.Sprintf("%s:%d", subName, constants.SubPortNumber), 0)
	if err != nil {
		return fmt.Errorf("error dialing %s", err)
	}
	defer subClient.Close()
	err = subclient.GetFiles(subClient, filesForMissingObjects,
		func(reader io.Reader, size uint64) error {
			hashVal, err := objAdderQueue.Add(reader, size)
			if err != nil {
				return err
			}
			delete(missingHashes, hashVal)
			return nil
		})
	if err != nil {
		return err
	}
	if len(missingHashes) > 0 {
		for hashVal := range missingHashes {
			fmt.Fprintf(os.Stderr, "Contents for file changed: %s\n",
				hashToFilename[hashVal])
		}
		return errors.New("one or more files on the sub changed")
	}
	return objAdderQueue.Close()
}
