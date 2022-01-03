package main

import (
	"fmt"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func traceInodeHistorySubcommand(args []string, logger log.DebugLogger) error {
	if err := traceInodeHistory(args[0], args[1]); err != nil {
		return fmt.Errorf("error tracing image inode: %s", err)
	}
	return nil
}

func getInodeInImage(img *image.Image, inodePath string) (
	filesystem.GenericInode, uint64, error) {
	fs := img.FileSystem
	filenameToInodeTable := fs.FilenameToInodeTable()
	if inum, ok := filenameToInodeTable[inodePath]; !ok {
		return nil, 0,
			fmt.Errorf("path: \"%s\" not present in image", inodePath)
	} else if inode, ok := fs.InodeTable[inum]; !ok {
		return nil, 0, fmt.Errorf("inode: %d not present in image", inum)
	} else {
		return inode, inum, nil
	}
}

func traceInodeHistory(imageName, inodePath string) error {
	imageSClient, _ := getClients()
	img, err := getImage(imageSClient, imageName)
	if err != nil {
		return err
	}
	sourceImageName := img.SourceImage
	if sourceImageName == "" {
		return fmt.Errorf("image: %s has no source: history cannot be traced",
			imageName)
	}
	lastImageName := imageName
	lastInode, inum, err := getInodeInImage(img, inodePath)
	if err != nil {
		return err
	}
	lastNumLinks := img.FileSystem.BuildNumLinksTable()[inum]
	for {
		if sourceImageName == "" {
			fmt.Printf("Inode originated in: %s\n", lastImageName)
			return listInode(lastInode, inodePath, lastNumLinks)
		}
		img, err = getImage(imageSClient, sourceImageName)
		if err != nil {
			return err
		}
		inode, inum, err := getInodeInImage(img, inodePath)
		if err != nil {
			fmt.Printf("Inode originated in: %s\n", lastImageName)
			return listInode(lastInode, inodePath, lastNumLinks)
		}
		sameType, sameMetadata, sameData := filesystem.CompareInodes(lastInode,
			inode, nil)
		if !sameType || !sameMetadata || !sameData {
			fmt.Printf("Inode changed in: %s\n", lastImageName)
			if e := listInode(lastInode, inodePath, lastNumLinks); e != nil {
				return e
			}
		}
		lastImageName = sourceImageName
		lastInode = inode
		lastNumLinks = img.FileSystem.BuildNumLinksTable()[inum]
		sourceImageName = img.SourceImage
	}
	return nil
}
