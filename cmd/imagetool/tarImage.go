package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/filesystem/tar"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
)

func tarImageSubcommand(args []string, logger log.DebugLogger) error {
	_, objectClient := getClients()
	outputFilename := ""
	if len(args) > 1 {
		outputFilename = args[1]
	}
	err := tarImageAndWrite(objectClient, args[0], outputFilename)
	if err != nil {
		return fmt.Errorf("error taring image: %s", err)
	}
	return nil
}

func tarImageAndWrite(objectClient *objectclient.ObjectClient, imageName,
	outputFilename string) error {
	fs, objectsGetter, _, err := getImageForUnpack(objectClient, imageName)
	if err != nil {
		return err
	}
	deleteOutfile := true
	output := os.Stdout
	if outputFilename != "" {
		output, err = os.Create(outputFilename)
		if err != nil {
			return err
		}
		defer func() {
			if deleteOutfile {
				output.Close()
				os.Remove(outputFilename)
			}
		}()
	}
	writer := bufio.NewWriter(output)
	if *compress {
		err = tarCompressed(writer, fs, objectsGetter)
	} else {
		err = tar.Write(writer, fs, objectsGetter)
	}
	if err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if output != os.Stdout {
		if err := output.Close(); err != nil {
			return err
		}
	}
	deleteOutfile = false
	return nil
}

func tarCompressed(writer io.Writer, fs *filesystem.FileSystem,
	objectsGetter objectserver.ObjectsGetter) error {
	zWriter := gzip.NewWriter(writer)
	if err := tar.Write(zWriter, fs, objectsGetter); err != nil {
		zWriter.Close()
		return err
	}
	return zWriter.Close()
}
