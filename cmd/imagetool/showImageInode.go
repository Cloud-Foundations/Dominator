package main

import (
	"fmt"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/filter"
	"github.com/Cloud-Foundations/Dominator/lib/log"
)

func showImageInodeSubcommand(args []string, logger log.DebugLogger) error {
	if err := showImageInode(args[0], args[1]); err != nil {
		return fmt.Errorf("error showing image inode: %s", err)
	}
	return nil
}

func showImageInode(image, inodePath string) error {
	fs, _, err := getTypedImage(image)
	if err != nil {
		return err
	}
	filenameToInodeTable := fs.FilenameToInodeTable()
	if inum, ok := filenameToInodeTable[inodePath]; !ok {
		return fmt.Errorf("path: \"%s\" not present in image", inodePath)
	} else if inode, ok := fs.InodeTable[inum]; !ok {
		return fmt.Errorf("inode: %d not present in image", inum)
	} else {
		filt, err := filter.New([]string{".*"})
		if err != nil {
			return err
		}
		numLinksTable := fs.BuildNumLinksTable()
		return inode.List(os.Stdout, inodePath, nil, numLinksTable[inum],
			listSelector, filt)
	}
}
