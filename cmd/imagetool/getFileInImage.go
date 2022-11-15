package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

func getFileInImageSubcommand(args []string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	var outFileName string
	if len(args) > 2 {
		outFileName = args[2]
	}
	err := getFileInImage(objectClient, args[0], args[1], outFileName)
	if err != nil {
		return fmt.Errorf("error getting file in image: %s", err)
	}
	return nil
}

func getFileInImage(objectClient *objectclient.ObjectClient, imageName,
	imageFile, outFileName string) error {
	if reader, err := getTypedFileReader(imageName, imageFile); err != nil {
		return err
	} else {
		defer reader.Close()
		if outFileName == "" {
			_, err := io.Copy(os.Stdout, reader)
			return err
		} else {
			return fsutil.CopyToFile(outFileName, filePerms, reader, 0)
		}
	}
}
