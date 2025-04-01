package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Cloud-Foundations/Dominator/imageserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	objectclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

func restoreImageSubcommand(args []string, logger log.DebugLogger) error {
	if err := restoreImage(args[0], logger); err != nil {
		return fmt.Errorf("error restoring image: %s", err)
	}
	return nil
}

func restoreImage(filename string, logger log.DebugLogger) error {
	masterImageSClient, _ := getMasterClients()
	imageSClient, _ := getClients()
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	decoder := gob.NewDecoder(reader)
	var imageArchive []byte
	if err := decoder.Decode(&imageArchive); err != nil {
		return err
	}
	var archive proto.ImageArchive
	err = gob.NewDecoder(bytes.NewReader(imageArchive)).Decode(&archive)
	if err != nil {
		return err
	}
	exists, err := client.CheckImage(masterImageSClient, archive.ImageName)
	if err != nil {
		return err
	} else if exists {
		return fmt.Errorf("%s already exists", archive.ImageName)
	}
	if !archive.ExpiresAt.IsZero() {
		if waitTime := time.Until(archive.ExpiresAt); waitTime >= 0 {
			return fmt.Errorf("cannot restore for: %s",
				format.Duration(waitTime))
		}
	}
	objQ, err := objectclient.NewObjectAdderQueue(imageSClient)
	if err != nil {
		return err
	}
	closeQueue := true
	defer func() {
		if closeQueue {
			objQ.Close()
		}
	}()
	for {
		var object objectType
		if err := decoder.Decode(&object); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if hashVal, err := objQ.Add(reader, object.Length); err != nil {
			return err
		} else if hashVal != object.Hash {
			return fmt.Errorf("corrupted object: %x", hashVal)
		}
	}
	closeQueue = false
	if err := objQ.Close(); err != nil {
		return err
	}
	timeout := *expiresIn
	if timeout < 1 {
		timeout = 15 * time.Minute
	}
	response, err := client.RestoreImageFromArchive(masterImageSClient,
		proto.RestoreImageFromArchiveRequest{
			ExpiresAt:   time.Now().Add(timeout),
			ArchiveData: imageArchive,
		})
	if err != nil {
		return err
	}
	if response.ReplicationMaster != "" {
		return fmt.Errorf("please specify replication master: %s",
			response.ReplicationMaster)
	}
	logger.Printf("restored image: %s\n", archive.ImageName)
	return nil
}
