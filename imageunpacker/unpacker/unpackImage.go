package unpacker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	domlib "github.com/Cloud-Foundations/Dominator/dom/lib"
	imageclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/util"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	unpackproto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
	subproto "github.com/Cloud-Foundations/Dominator/proto/sub"
	sublib "github.com/Cloud-Foundations/Dominator/sub/lib"
)

func (u *Unpacker) unpackImage(streamName string, imageLeafName string) error {
	u.updateUsageTime()
	defer u.updateUsageTime()
	streamInfo := u.getStream(streamName)
	if streamInfo == nil {
		return errors.New("unknown stream")
	}
	imageName := filepath.Join(streamName, imageLeafName)
	fs := u.getImage(imageName, streamInfo.dualLogger).FileSystem
	if err := fs.RebuildInodePointers(); err != nil {
		return err
	}
	fs.InodeToFilenamesTable()
	fs.FilenameToInodeTable()
	fs.HashToInodesTable()
	fs.ComputeTotalDataBytes()
	fs.BuildEntryMap()
	errorChannel := make(chan error)
	request := requestType{
		request:      requestUnpack,
		desiredFS:    fs,
		imageName:    imageName,
		errorChannel: errorChannel,
	}
	streamInfo.requestChannel <- request
	return <-errorChannel
}

func (u *Unpacker) getImage(imageName string,
	logger log.DebugLogger) *image.Image {
	logger.Printf("Getting image: %s\n", imageName)
	interval := time.Second
	for ; true; time.Sleep(interval) {
		srpcClient, err := srpc.DialHTTP("tcp", u.imageServerAddress,
			time.Second*15)
		if err != nil {
			logger.Printf("Error connecting to image server: %s\n", err)
			continue
		}
		image, err := imageclient.GetImageWithTimeout(srpcClient, imageName,
			time.Minute)
		srpcClient.Close()
		if err != nil {
			logger.Printf("Error getting image: %s\n", err)
			continue
		}
		if image != nil {
			return image
		}
		logger.Printf("Image: %s not ready yet\n", imageName)
		if interval < time.Second*10 {
			interval += time.Second
		}
	}
	return nil
}

func (stream *streamManagerState) unpack(imageName string,
	desiredFS *filesystem.FileSystem) error {
	srpcClient, err := srpc.DialHTTP("tcp", stream.unpacker.imageServerAddress,
		time.Second*15)
	if err != nil {
		return err
	}
	defer srpcClient.Close()
	objectServer := objectclient.AttachObjectClient(srpcClient)
	defer objectServer.Close()
	mountPoint := filepath.Join(stream.unpacker.baseDir, "mnt")
	streamInfo := stream.streamInfo
	switch streamInfo.status {
	case unpackproto.StatusStreamScanned:
		// Everything is set up. Ready to unpack.
	case unpackproto.StatusStreamNoFileSystem:
		err := stream.mkfs(desiredFS, objectServer, streamInfo.dualLogger)
		if err != nil {
			return err
		}
		if err := stream.scan(false); err != nil {
			return err
		}
	default:
		return errors.New("not yet scanned")
	}
	err = stream.deleteUnneededFiles(imageName, stream.fileSystem, desiredFS,
		mountPoint)
	if err != nil {
		return err
	}
	subObj := domlib.Sub{
		FileSystem:  stream.fileSystem,
		ObjectCache: stream.objectCache,
	}
	stream.fileSystem = nil
	emptyFilter, _ := filter.New(nil)
	desiredImage := &image.Image{FileSystem: desiredFS, Filter: emptyFilter}
	fetchMap, _ := domlib.BuildMissingLists(subObj, desiredImage, false,
		true, streamInfo.dualLogger)
	objectsToFetch := objectcache.ObjectMapToCache(fetchMap)
	objectsDir := filepath.Join(mountPoint, ".subd", "objects")
	err = stream.fetch(imageName, objectsToFetch, objectsDir, objectServer)
	if err != nil {
		streamInfo.status = unpackproto.StatusStreamMounted
		return err
	}
	subObj.ObjectCache = append(subObj.ObjectCache, objectsToFetch...)
	streamInfo.status = unpackproto.StatusStreamUpdating
	streamInfo.dualLogger.Printf("Update(%s) starting\n", imageName)
	startTime := time.Now()
	var request subproto.UpdateRequest
	domlib.BuildUpdateRequest(subObj, desiredImage, &request, true, false,
		streamInfo.dualLogger)
	_, _, err = sublib.Update(request, mountPoint, objectsDir, nil, nil, nil,
		streamInfo.streamLogger)
	if err == nil {
		err = util.WriteImageName(mountPoint, imageName)
	}
	streamInfo.status = unpackproto.StatusStreamMounted
	streamInfo.dualLogger.Printf("Update(%s) completed in %s\n",
		imageName, format.Duration(time.Since(startTime)))
	return err
}

func (stream *streamManagerState) deleteUnneededFiles(imageName string,
	subFS, imgFS *filesystem.FileSystem, mountPoint string) error {
	pathsToDelete := make([]string, 0)
	imgHashToInodesTable := imgFS.HashToInodesTable()
	imgFilenameToInodeTable := imgFS.FilenameToInodeTable()
	for pathname, inum := range subFS.FilenameToInodeTable() {
		if inode, ok := subFS.InodeTable[inum].(*filesystem.RegularInode); ok {
			if inode.Size > 0 {
				if _, ok := imgHashToInodesTable[inode.Hash]; !ok {
					pathsToDelete = append(pathsToDelete, pathname)
				}
			} else {
				if _, ok := imgFilenameToInodeTable[pathname]; !ok {
					pathsToDelete = append(pathsToDelete, pathname)
				}
			}
		}
	}
	if len(pathsToDelete) < 1 {
		return nil
	}
	streamInfo := stream.streamInfo
	streamInfo.dualLogger.Printf("Deleting(%s): %d unneeded files\n",
		imageName, len(pathsToDelete))
	for _, pathname := range pathsToDelete {
		streamInfo.streamLogger.Printf("Delete(%s): %s\n", imageName, pathname)
		os.Remove(filepath.Join(mountPoint, pathname))
	}
	return nil
}

func (stream *streamManagerState) fetch(imageName string,
	objectsToFetch []hash.Hash, destDirname string,
	objectsGetter objectserver.ObjectsGetter) error {
	startTime := time.Now()
	stream.streamInfo.status = unpackproto.StatusStreamFetching
	objectsReader, err := objectsGetter.GetObjects(objectsToFetch)
	if err != nil {
		stream.streamInfo.status = unpackproto.StatusStreamMounted
		return err
	}
	defer objectsReader.Close()
	streamInfo := stream.streamInfo
	streamInfo.dualLogger.Printf("Fetching(%s) %d objects\n",
		imageName, len(objectsToFetch))
	var totalBytes uint64
	for _, hashVal := range objectsToFetch {
		length, reader, err := objectsReader.NextObject()
		if err != nil {
			streamInfo.dualLogger.Println(err)
			stream.streamInfo.status = unpackproto.StatusStreamMounted
			return err
		}
		err = readOne(destDirname, hashVal, length, reader)
		reader.Close()
		if err != nil {
			streamInfo.dualLogger.Println(err)
			stream.streamInfo.status = unpackproto.StatusStreamMounted
			return err
		}
		totalBytes += length
	}
	timeTaken := time.Since(startTime)
	streamInfo.dualLogger.Printf("Fetched(%s) %d objects, %s in %s (%s/s)\n",
		imageName, len(objectsToFetch), format.FormatBytes(totalBytes),
		format.Duration(timeTaken),
		format.FormatBytes(uint64(float64(totalBytes)/timeTaken.Seconds())))
	return nil
}

func (stream *streamManagerState) mkfs(fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter, logger log.Logger) error {
	unsupportedOptions, err := util.GetUnsupportedExt4fsOptions(fs,
		objectsGetter)
	if err != nil {
		return err
	}
	stream.unpacker.rwMutex.RLock()
	device := stream.unpacker.pState.Devices[stream.streamInfo.DeviceId]
	stream.unpacker.rwMutex.RUnlock()
	// udev has a bug where the partition device node is created and sometimes
	// is removed and then created again. Based on experiments the device node
	// is gone for ~15 milliseconds. Wait long enough since the partition was
	// created to hopefully never encounter this race again.
	if !device.partitionTimestamp.IsZero() {
		timeSincePartition := time.Since(device.partitionTimestamp)
		if timeSincePartition < time.Second {
			sleepTime := time.Second - timeSincePartition
			logger.Printf("sleeping %s to work around udev race\n",
				format.Duration(sleepTime))
			time.Sleep(sleepTime)
		}
	}
	partitionPath, err := getPartition(filepath.Join("/dev", device.DeviceName))
	if err != nil {
		return err
	}
	rootLabel := fmt.Sprintf("rootfs@%x", time.Now().Unix())
	err = util.MakeExt4fs(partitionPath, rootLabel, unsupportedOptions, 8192,
		logger)
	if err != nil {
		return err
	}
	// Make sure it's still a block device. If not it means udev still had not
	// settled down after waiting, so remove the inode and return an error.
	if err := checkIfBlockDevice(partitionPath); err != nil {
		os.Remove(partitionPath)
		return err
	}
	stream.streamInfo.status = unpackproto.StatusStreamNotMounted
	stream.rootLabel = rootLabel
	return nil
}

func checkIfBlockDevice(path string) error {
	if fi, err := os.Lstat(path); err != nil {
		return err
	} else if fi.Mode()&os.ModeType != os.ModeDevice {
		return fmt.Errorf("%s is not a device, mode: %s", path, fi.Mode())
	}
	return nil
}

func getPartition(devicePath string) (string, error) {
	partitionPaths := []string{devicePath + "1", devicePath + "p1"}
	for _, partitionPath := range partitionPaths {
		if err := checkIfBlockDevice(partitionPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if file, err := os.Open(partitionPath); err == nil {
			file.Close()
			return partitionPath, nil
		}
	}
	return "", fmt.Errorf("no partitions found for: %s", devicePath)
}

func readOne(objectsDir string, hashVal hash.Hash, length uint64,
	reader io.Reader) error {
	filename := filepath.Join(objectsDir, objectcache.HashToFilename(hashVal))
	dirname := filepath.Dir(filename)
	if err := os.MkdirAll(dirname, dirPerms); err != nil {
		return err
	}
	return fsutil.CopyToFile(filename, filePerms, reader, length)
}
