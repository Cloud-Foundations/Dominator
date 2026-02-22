package main

import (
	"errors"
	"fmt"
	"os"

	imgclient "github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageInodeSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageInode(args[0], args[1]); err != nil {
		return fmt.Errorf("error showing image inode: %s", err)
	}
	return nil
}

func listInode(inode filesystem.GenericInode, inodePath string,
	numLinks int) error {
	filt, err := filter.New([]string{".*"}) // Do not descend directory.
	if err != nil {
		return err
	}
	return inode.List(os.Stdout, inodePath, nil, numLinks,
		filesystem.ListParams{Filter: filt,
			ListSelector: listSelector,
		})
}

func showImageInode(image, inodePath string) error {
	done, err := showImageInodeFast(image, filesystem.Filename(inodePath))
	if err != nil {
		return err
	} else if done {
		return nil
	}
	fs, err := getTypedFileSystem(image)
	if err != nil {
		return err
	}
	filenameToInodeTable := fs.FilenameToInodeTable()
	if inum, ok := filenameToInodeTable[inodePath]; !ok {
		return fmt.Errorf("path: \"%s\" not present in image", inodePath)
	} else if inode, ok := fs.InodeTable[inum]; !ok {
		return fmt.Errorf("inode: %d not present in image", inum)
	} else {
		numLinksTable := fs.BuildNumLinksTable()
		return listInode(inode, inodePath, numLinksTable[inum])
	}
}

func showImageInodeFast(imageName string, inodePath filesystem.Filename) (
	bool, error) {
	imageSClient, _ := getClients()
	imType, err := makeTypedImage(imageName)
	if err != nil {
		return false, err
	}
	switch imType.imageType {
	case imageTypeImage:
		imageName = imType.specifier
	case imageTypeLatestImage:
		name, err := imgclient.FindLatestImage(imageSClient, imType.specifier,
			false)
		if err != nil {
			return false, err
		}
		if name == "" {
			return false, errors.New(imageName + ": not found")
		}
		imageName = name
	default:
		return false, nil
	}
	response, err := imgclient.GetImageInodes(
		imageSClient,
		imageName,
		[]filesystem.Filename{inodePath})
	if err != nil {
		return false, nil // Fall back to the slow way.
	}
	if inum, ok := response.InodeNumbers[inodePath]; !ok {
		return false, fmt.Errorf("path: \"%s\" not present in image", inodePath)
	} else if inode, ok := response.Inodes[inum]; !ok {
		return false, fmt.Errorf("inode: %d not present in image", inum)
	} else {
		return true, listInode(inode, string(inodePath),
			int(response.NumLinks[inum]))
	}
}
