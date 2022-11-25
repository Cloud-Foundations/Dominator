package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

func getImageBuildLogSubcommand(args []string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	var outFileName string
	if len(args) > 1 {
		outFileName = args[1]
	}
	err := getImageBuildLog(objectClient, args[0], outFileName)
	if err != nil {
		return fmt.Errorf("error getting image build log: %s", err)
	}
	return nil
}

func getImageBuildLog(objectClient *objectclient.ObjectClient,
	imageName, outFileName string) error {
	reader, err := getTypedImageBuildLogReader(imageName)
	if err != nil {
		return err
	}
	defer reader.Close()
	if outFileName == "" {
		_, err := io.Copy(os.Stdout, reader)
		return err
	} else {
		return fsutil.CopyToFile(outFileName, filePerms, reader, 0)
	}
}
