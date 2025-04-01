package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

// Multiple objectType objects are encoded after the encoded proto.ImageArchive.
type objectType struct {
	Hash   hash.Hash
	Length uint64
} // Object data are streamed afterwards.

func saveImageSubcommand(args []string, logger log.DebugLogger) error {
	var outFileName string
	if len(args) > 1 {
		outFileName = args[1]
	}
	if err := saveImage(args[0], outFileName, logger); err != nil {
		return fmt.Errorf("error saving image: %s", err)
	}
	return nil
}

func saveImage(imageName string, outFileName string,
	logger log.DebugLogger) error {
	imageSClient, _ := getMasterClients()
	_, objectClient := getClients()
	response, err := client.GetImageArchive(imageSClient, imageName)
	if err != nil {
		return err
	}
	if response.ReplicationMaster != "" {
		return fmt.Errorf("please specify replication master: %s",
			response.ReplicationMaster)
	}
	decoder := gob.NewDecoder(bytes.NewReader(response.ArchiveData))
	var img proto.ImageArchive
	if err := decoder.Decode(&img); err != nil {
		return err
	}
	var writer *bufio.Writer
	if outFileName == "" {
		writer = bufio.NewWriter(os.Stdout)
	} else {
		file, err := os.Create(outFileName)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = bufio.NewWriter(file)
	}
	defer writer.Flush()
	encoder := gob.NewEncoder(writer)
	if err := encoder.Encode(response.ArchiveData); err != nil {
		return err
	}
	objectsMap := make(map[hash.Hash]struct{})
	var objectsList []hash.Hash
	img.ForEachObject(func(object hash.Hash) error {
		if _, ok := objectsMap[object]; !ok {
			objectsMap[object] = struct{}{}
			objectsList = append(objectsList, object)
		}
		return nil
	})
	objectsReader, err := objectClient.GetObjects(objectsList)
	if err != nil {
		return err
	}
	defer objectsReader.Close()
	for _, hashVal := range objectsList {
		err := saveObject(writer, encoder, objectsReader, hashVal)
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

func saveObject(writer io.Writer, encoder *gob.Encoder,
	objectsReader objectserver.ObjectsReader, hashVal hash.Hash) error {
	receivedLength, readCloser, err := objectsReader.NextObject()
	if err != nil {
		return err
	}
	defer readCloser.Close()
	object := objectType{
		Hash:   hashVal,
		Length: receivedLength,
	}
	if err := encoder.Encode(object); err != nil {
		return err
	}
	_, err = io.Copy(writer, readCloser)
	return err
}
