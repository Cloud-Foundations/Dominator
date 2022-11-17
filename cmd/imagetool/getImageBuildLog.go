package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
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
	img, err := getTypedImageMetadata(imageName)
	if err != nil {
		return err
	}
	buildLog := img.BuildLog
	if buildLog == nil {
		return errors.New("no build log")
	}
	var reader io.Reader
	var size uint64
	if hashPtr := buildLog.Object; hashPtr != nil {
		s, r, err := objectClient.GetObject(*hashPtr)
		if err != nil {
			return err
		}
		defer r.Close()
		reader = r
		size = s
	} else if buildLog.URL != "" {
		resp, err := http.Get(buildLog.URL)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return errors.New(resp.Status)
		}
		defer resp.Body.Close()
		reader = resp.Body
		if resp.ContentLength > 0 {
			size = uint64(resp.ContentLength)
		}
	} else {
		return errors.New("no build log data")
	}
	if outFileName == "" {
		_, err := io.Copy(os.Stdout, reader)
		return err
	} else {
		return fsutil.CopyToFile(outFileName, filePerms, reader, size)
	}
}
