package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/scanner"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/untar"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/mbr"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/triggers"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

type hasher struct {
	objQ *objectclient.ObjectAdderQueue
}

func addImage(imageSClient *srpc.Client, name string, img *image.Image,
	logger log.DebugLogger) error {
	if *expiresIn > 0 {
		img.ExpiresAt = time.Now().Add(*expiresIn)
	} else {
		img.ExpiresAt = time.Time{}
	}
	if err := img.Verify(); err != nil {
		return err
	}
	if err := img.VerifyRequiredPaths(requiredPaths); err != nil {
		return err
	}
	startTime := time.Now()
	if err := client.AddImage(imageSClient, name, img); err != nil {
		return errors.New("remote error: " + err.Error())
	}
	logger.Debugf(0, "Uploaded image: %s in %s\n",
		name, format.Duration(time.Since(startTime)))
	return nil
}

func (h *hasher) Hash(reader io.Reader, length uint64) (
	hash.Hash, error) {
	startTime := time.Now()
	hashVal, err := h.objQ.Add(reader, length)
	if err != nil {
		return hashVal, errors.New("error sending image data: " + err.Error())
	}
	if *objectAddInterval > 0 {
		time.Sleep(time.Until(startTime.Add(*objectAddInterval)))
	}
	return hashVal, nil
}

func buildImage(imageSClient *srpc.Client, filter *filter.Filter,
	imageFilename string,
	logger log.DebugLogger) (*filesystem.FileSystem, error) {
	var h hasher
	var err error
	h.objQ, err = objectclient.NewObjectAdderQueue(imageSClient)
	if err != nil {
		return nil, err
	}
	startTime := time.Now()
	fs, err := buildImageWithHasher(imageSClient, filter, imageFilename, &h)
	if err != nil {
		h.objQ.Close()
		return nil, err
	}
	err = h.objQ.Close()
	if err != nil {
		return nil, err
	}
	duration := time.Since(startTime)
	speed := uint64(float64(fs.TotalDataBytes) / duration.Seconds())
	logger.Debugf(0,
		"Scanned file-system and uploaded %d objects (%s) in %s (%s/s)\n",
		fs.NumRegularInodes, format.FormatBytes(fs.TotalDataBytes),
		format.Duration(duration), format.FormatBytes(speed))
	return fs, nil
}

func buildImageWithHasher(imageSClient *srpc.Client, filter *filter.Filter,
	imageFilename string, h scanner.Hasher) (*filesystem.FileSystem, error) {
	fi, err := os.Lstat(imageFilename)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		startTime := time.Now()
		sfs, err := scanner.ScanFileSystemWithParams(scanner.Params{
			RootDirectoryName: imageFilename,
			Runner:            concurrent.NewAutoScaler(0),
			ScanFilter:        filter,
			Hasher:            h,
		})
		if err != nil {
			return nil, err
		}
		logger.Debugf(0, "scanned: %s in %s\n",
			imageFilename, format.Duration(time.Since(startTime)))
		return &sfs.FileSystem, nil
	}
	imageFile, err := os.Open(imageFilename)
	if err != nil {
		return nil, errors.New("error opening image file: " + err.Error())
	}
	defer imageFile.Close()
	if partitionTable, err := mbr.Decode(imageFile); err != nil {
		if err != io.EOF {
			return nil, err
		} // Else perhaps a tiny tarfile, definitely not a partition table.
	} else if partitionTable != nil {
		return buildImageFromRaw(imageSClient, filter, imageFile,
			partitionTable, h)
	}
	var imageReader io.Reader
	if strings.HasSuffix(imageFilename, ".tar") {
		imageReader = imageFile
	} else if strings.HasSuffix(imageFilename, ".tar.gz") ||
		strings.HasSuffix(imageFilename, ".tgz") {
		gzipReader, err := gzip.NewReader(imageFile)
		if err != nil {
			return nil, errors.New("error creating gzip reader: " + err.Error())
		}
		defer gzipReader.Close()
		imageReader = gzipReader
	} else {
		return nil, errors.New("unrecognised image type")
	}
	tarReader := tar.NewReader(imageReader)
	fs, err := untar.Decode(tarReader, h, filter)
	if err != nil {
		return nil, errors.New("error building image: " + err.Error())
	}
	return fs, nil
}

func buildImageFromRaw(imageSClient *srpc.Client, filter *filter.Filter,
	imageFile *os.File, partitionTable *mbr.Mbr,
	h scanner.Hasher) (*filesystem.FileSystem, error) {
	var index uint
	var offsetOfLargest, sizeOfLargest uint64
	numPartitions := partitionTable.GetNumPartitions()
	for index = 0; index < numPartitions; index++ {
		offset := partitionTable.GetPartitionOffset(index)
		size := partitionTable.GetPartitionSize(index)
		if size > sizeOfLargest {
			offsetOfLargest = offset
			sizeOfLargest = size
		}
	}
	if sizeOfLargest < 1 {
		return nil, errors.New("unable to find largest partition")
	}
	if err := wsyscall.UnshareMountNamespace(); err != nil {
		if os.IsPermission(err) {
			// Try again with sudo(8).
			return nil, reExecAsRoot()
		}
		return nil, errors.New(
			"error unsharing mount namespace: " + err.Error())
	}
	cmd := exec.Command("mount", "-o",
		fmt.Sprintf("loop,offset=%d", offsetOfLargest), imageFile.Name(),
		"/mnt")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	fs, err := buildImageWithHasher(imageSClient, filter, "/mnt", h)
	wsyscall.Unmount("/mnt", 0)
	return fs, err
}

func loadImageFiles(img *image.Image, objectClient *objectclient.ObjectClient,
	filterFilename, triggersFilename string) error {
	var err error
	if filterFilename != "" {
		img.Filter, err = filter.Load(filterFilename)
		if err != nil {
			return err
		}
	}
	if triggersFilename != "" {
		img.Triggers, err = triggers.Load(triggersFilename)
		if err != nil {
			return err
		}
	}
	img.BuildLog, err = getAnnotation(objectClient, *buildLog)
	if err != nil {
		return err
	}
	img.ReleaseNotes, err = getAnnotation(objectClient, *releaseNotes)
	if err != nil {
		return err
	}
	return nil
}

func getAnnotation(objectClient *objectclient.ObjectClient, name string) (
	*image.Annotation, error) {
	if name == "" {
		return nil, nil
	}
	file, err := os.Open(name)
	if err != nil {
		return &image.Annotation{URL: name}, nil
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	hash, _, err := objectClient.AddObject(reader, uint64(fi.Size()), nil)
	return &image.Annotation{Object: &hash}, err
}
