package main

import (
	"fmt"
	"os"

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
	filt, err := filter.New([]string{".*"})
	if err != nil {
		return err
	}
	return inode.List(os.Stdout, inodePath, nil, numLinks, listSelector, filt)
}

func showImageInode(image, inodePath string) error {
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
