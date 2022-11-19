package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	hyperclient "github.com/Cloud-Foundations/Dominator/hypervisor/client"
	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/constants"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/image/packageutil"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	fm_proto "github.com/Cloud-Foundations/Dominator/proto/fleetmanager"
	"github.com/Cloud-Foundations/Dominator/proto/sub"
	subclient "github.com/Cloud-Foundations/Dominator/sub/client"
)

const (
	imageTypeDirectory = iota
	imageTypeFileSystem
	imageTypeImage
	imageTypeLatestImage
	imageTypeImageFile
	imageTypeSub
	imageTypeVM
)

type typedImage struct {
	fileSystem *filesystem.FileSystem
	filter     *filter.Filter
	image      *image.Image
	imageType  uint
	specifier  string
}

func getTypedFileReader(typedName, filename string) (io.ReadCloser, error) {
	ti, err := makeTypedImage(typedName)
	if err != nil {
		return nil, err
	}
	return ti.openFile(filename)
}

func getTypedFileSystem(typedName string) (*filesystem.FileSystem, error) {
	ti, err := getTypedImageType(typedName)
	if err != nil {
		return nil, err
	}
	fs, err := ti.getFileSystem()
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func getTypedFileSystemAndFilter(typedName string) (
	*filesystem.FileSystem, *filter.Filter, error) {
	ti, err := getTypedImageType(typedName)
	if err != nil {
		return nil, nil, err
	}
	fs, err := ti.getFileSystem()
	if err != nil {
		return nil, nil, err
	}
	return fs, ti.filter, nil
}

func getTypedImage(typedName string) (*image.Image, error) {
	ti, err := getTypedImageType(typedName)
	if err != nil {
		return nil, err
	}
	img, err := ti.getImage()
	if err != nil {
		return nil, err
	}
	return img, nil
}

func getTypedImageFilter(typedName string) (*filter.Filter, error) {
	ti, err := makeTypedImage(typedName)
	if err != nil {
		return nil, err
	}
	if err := ti.loadMetadata(); err != nil {
		return nil, err
	}
	filt, err := ti.getFilter()
	if err != nil {
		return nil, err
	}
	return filt, nil
}

func getTypedImageMetadata(typedName string) (*image.Image, error) {
	ti, err := makeTypedImage(typedName)
	if err != nil {
		return nil, err
	}
	if err := ti.loadMetadata(); err != nil {
		return nil, err
	}
	img, err := ti.getImage()
	if err != nil {
		return nil, err
	}
	return img, nil
}

func getTypedImageType(typedName string) (*typedImage, error) {
	ti, err := makeTypedImage(typedName)
	if err != nil {
		return nil, err
	}
	if err := ti.load(); err != nil {
		return nil, err
	}
	return ti, nil
}

func getTypedPackageList(typedName string) ([]image.Package, error) {
	ti, err := makeTypedImage(typedName)
	if err != nil {
		return nil, err
	}
	return ti.loadPackages()
}

func makeTypedImage(typedName string) (*typedImage, error) {
	if len(typedName) < 3 || typedName[1] != ':' {
		typedName = "i:" + typedName
	}
	var retval *typedImage
	switch name := typedName[2:]; typedName[0] {
	case 'd':
		retval = &typedImage{imageType: imageTypeDirectory, specifier: name}
	case 'f':
		retval = &typedImage{imageType: imageTypeFileSystem, specifier: name}
	case 'i':
		retval = &typedImage{imageType: imageTypeImage, specifier: name}
	case 'I':
		retval = &typedImage{imageType: imageTypeLatestImage, specifier: name}
	case 'l':
		retval = &typedImage{imageType: imageTypeImageFile, specifier: name}
	case 's':
		retval = &typedImage{imageType: imageTypeSub, specifier: name}
	case 'v':
		retval = &typedImage{imageType: imageTypeVM, specifier: name}
	default:
		return nil, errors.New("unknown image type: " + typedName[:1])
	}
	return retval, nil
}

func (ti *typedImage) getFileSystem() (*filesystem.FileSystem, error) {
	if fs := ti.fileSystem; fs == nil {
		return nil, errors.New("FileSystem data not available")
	} else {
		return fs, nil
	}
}

func (ti *typedImage) getFilter() (*filter.Filter, error) {
	if filt := ti.filter; filt == nil {
		return nil, errors.New("Filter not available")
	} else {
		return filt, nil
	}
}

func (ti *typedImage) getImage() (*image.Image, error) {
	if img := ti.image; img == nil {
		return nil, errors.New("Image data not available")
	} else {
		return img, nil
	}
}

func (ti *typedImage) load() error {
	switch ti.imageType {
	case imageTypeDirectory:
		fs, err := scanDirectory(ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = fs
	case imageTypeFileSystem:
		fs, err := readFileSystem(ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = fs
	case imageTypeImage:
		imageSClient, _ := getClients()
		img, err := getImage(imageSClient, ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = img.FileSystem
		ti.filter = img.Filter
		ti.image = img
	case imageTypeLatestImage:
		imageSClient, _ := getClients()
		img, err := getLatestImage(imageSClient, ti.specifier, false)
		if err != nil {
			return err
		}
		ti.fileSystem = img.FileSystem
		ti.filter = img.Filter
		ti.image = img
	case imageTypeImageFile:
		img, err := readImage(ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = img.FileSystem
		ti.filter = img.Filter
		ti.image = img
	case imageTypeSub:
		fs, err := pollImage(ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = fs
	case imageTypeVM:
		fs, err := scanVm(ti.specifier)
		if err != nil {
			return err
		}
		ti.fileSystem = fs
	default:
		panic("unsupported typedImage in load()")
	}
	return nil
}

func (ti *typedImage) loadMetadata() error {
	switch ti.imageType {
	case imageTypeImage:
		img, err := getImageMetadata(ti.specifier)
		if err != nil {
			return err
		}
		ti.filter = img.Filter
		ti.image = img
	case imageTypeLatestImage:
		imageSClient, _ := getClients()
		img, err := getLatestImage(imageSClient, ti.specifier, true)
		if err != nil {
			return err
		}
		ti.filter = img.Filter
		ti.image = img
	case imageTypeImageFile:
		img, err := readImage(ti.specifier)
		if err != nil {
			return err
		}
		ti.filter = img.Filter
		ti.image = img
	default:
		return errors.New("package data not available")
	}
	return nil
}

func (ti *typedImage) loadPackages() ([]image.Package, error) {
	switch ti.imageType {
	case imageTypeDirectory:
		return packageutil.GetPackageList(func(cmd string, w io.Writer) error {
			command := exec.Command("/bin/generic-packager", cmd)
			command.Stdout = w
			return command.Run()
		})
	case imageTypeImage, imageTypeLatestImage, imageTypeImageFile:
		if err := ti.loadMetadata(); err != nil {
			return nil, err
		}
		return ti.image.Packages, nil
	default:
		return nil, errors.New("package data not available")
	}
}

func (ti *typedImage) openFile(filename string) (io.ReadCloser, error) {
	switch ti.imageType {
	case imageTypeDirectory:
		return os.Open(filepath.Join(ti.specifier, filename))
	case imageTypeFileSystem, imageTypeImage, imageTypeLatestImage, imageTypeImageFile:
		if err := ti.load(); err != nil {
			return nil, err
		}
	case imageTypeSub:
		data, err := readFileFromSub(ti.specifier, filename)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	default:
		return nil, errors.New("unsupported typedImage in openFile()")
	}
	fs, err := ti.getFileSystem()
	if err != nil {
		return nil, err
	}
	filenameToInodeTable := fs.FilenameToInodeTable()
	if inum, ok := filenameToInodeTable[filename]; !ok {
		return nil, fmt.Errorf("file: \"%s\" not present in image", filename)
	} else if inode, ok := fs.InodeTable[inum]; !ok {
		return nil, fmt.Errorf("inode: %d not present in image", inum)
	} else if inode, ok := inode.(*filesystem.RegularInode); !ok {
		return nil, fmt.Errorf("file: \"%s\" is not a regular file", filename)
	} else {
		_, objectClient := getClients()
		_, reader, err := objectClient.GetObject(inode.Hash)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}
}

func findHypervisor(vmIpAddr net.IP) (string, error) {
	if *hypervisorHostname != "" {
		return fmt.Sprintf("%s:%d", *hypervisorHostname, *hypervisorPortNum),
			nil
	} else if *fleetManagerHostname != "" {
		fm := fmt.Sprintf("%s:%d", *fleetManagerHostname, *fleetManagerPortNum)
		client, err := srpc.DialHTTP("tcp", fm, time.Second*10)
		if err != nil {
			return "", err
		}
		defer client.Close()
		return findHypervisorClient(client, vmIpAddr)
	} else {
		return fmt.Sprintf("localhost:%d", *hypervisorPortNum), nil
	}
}

func findHypervisorClient(client *srpc.Client,
	vmIpAddr net.IP) (string, error) {
	request := fm_proto.GetHypervisorForVMRequest{vmIpAddr}
	var reply fm_proto.GetHypervisorForVMResponse
	err := client.RequestReply("FleetManager.GetHypervisorForVM", request,
		&reply)
	if err != nil {
		return "", err
	}
	if err := errors.New(reply.Error); err != nil {
		return "", err
	}
	return reply.HypervisorAddress, nil
}

func getImage(client *srpc.Client, name string) (*image.Image, error) {
	img, err := imgclient.GetImageWithTimeout(client, name, *timeout)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, errors.New(name + ": not found")
	}
	if err := img.FileSystem.RebuildInodePointers(); err != nil {
		return nil, err
	}
	return img, nil
}

func getLatestImage(client *srpc.Client, name string,
	ignoreFilesystem bool) (*image.Image, error) {
	imageName, err := imgclient.FindLatestImage(client, name, *ignoreExpiring)
	if err != nil {
		return nil, err
	}
	if ignoreFilesystem {
		return getImageMetadata(imageName)
	}
	return getImage(client, imageName)
}

func getVmIpAndHypervisor(vmHostname string) (net.IP, *srpc.Client, error) {
	vmIpAddr, err := lookupIP(vmHostname)
	if err != nil {
		return nil, nil, err
	}
	hypervisorAddress, err := findHypervisor(vmIpAddr)
	if err != nil {
		return nil, nil, err
	}
	client, err := srpc.DialHTTP("tcp", hypervisorAddress, time.Second*10)
	if err != nil {
		return nil, nil, err
	}
	return vmIpAddr, client, nil
}

func lookupIP(vmHostname string) (net.IP, error) {
	if ips, err := net.LookupIP(vmHostname); err != nil {
		return nil, err
	} else if len(ips) != 1 {
		return nil, fmt.Errorf("num IPs: %d != 1", len(ips))
	} else {
		return ips[0], nil
	}
}

func pollImage(name string) (*filesystem.FileSystem, error) {
	clientName := fmt.Sprintf("%s:%d", name, constants.SubPortNumber)
	srpcClient, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return nil, fmt.Errorf("error dialing %s", err)
	}
	defer srpcClient.Close()
	var request sub.PollRequest
	var reply sub.PollResponse
	if err = subclient.CallPoll(srpcClient, request, &reply); err != nil {
		return nil, err
	}
	if reply.FileSystem == nil {
		return nil, errors.New("no poll data")
	}
	reply.FileSystem.RebuildInodePointers()
	return reply.FileSystem, nil
}

func readFileFromSub(subHostname, filename string) ([]byte, error) {
	clientName := fmt.Sprintf("%s:%d", subHostname, constants.SubPortNumber)
	srpcClient, err := srpc.DialHTTP("tcp", clientName, 0)
	if err != nil {
		return nil, fmt.Errorf("error dialing %s", err)
	}
	defer srpcClient.Close()
	buffer := &bytes.Buffer{}
	err = subclient.GetFiles(srpcClient, []string{filename},
		func(reader io.Reader, size uint64) error {
			_, err := io.Copy(buffer, reader)
			return err
		})
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func readFileSystem(name string) (*filesystem.FileSystem, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var fileSystem filesystem.FileSystem
	if err := gob.NewDecoder(file).Decode(&fileSystem); err != nil {
		return nil, err
	}
	fileSystem.RebuildInodePointers()
	return &fileSystem, nil
}

func readImage(name string) (*image.Image, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var img image.Image
	if err := gob.NewDecoder(file).Decode(&img); err != nil {
		return nil, err
	}
	img.FileSystem.RebuildInodePointers()
	return &img, nil
}

func scanDirectory(name string) (*filesystem.FileSystem, error) {
	fs, err := buildImageWithHasher(nil, nil, name, nil)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func scanVm(name string) (*filesystem.FileSystem, error) {
	vmIpAddr, srpcClient, err := getVmIpAndHypervisor(name)
	if err != nil {
		return nil, err
	}
	defer srpcClient.Close()
	fs, err := hyperclient.ScanVmRoot(srpcClient, vmIpAddr, nil)
	if err != nil {
		return nil, err
	}
	fs.RebuildInodePointers()
	return fs, nil
}
