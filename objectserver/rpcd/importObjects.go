package rpcd

import (
	"fmt"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/concurrent"
	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/net/http"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

const (
	maxConcurrency = 128
)

var (
	httpClient = http.NewLimitedConcurrencyClient(
		make(chan struct{}, maxConcurrency))
)

func (t *srpcType) ImportObjects(conn *srpc.Conn,
	request objectserver.ImportObjectsRequest,
	reply *objectserver.ImportObjectsResponse) error {
	if t.replicationMaster != "" {
		reply.Error = replicationMessage + t.replicationMaster
		return nil
	}
	t.logger.Printf("ImportObjects() adding: %d objects by: %s from: %s\n",
		len(request.Hashes), conn.Username(), request.BaseRemoteUrl)
	err := t.importObjects(request, reply)
	reply.Error = errors.ErrorToString(err)
	return nil
}

func (t *srpcType) importObjects(request objectserver.ImportObjectsRequest,
	reply *objectserver.ImportObjectsResponse) error {
	foundSizes, err := t.objectServer.CheckObjects(request.Hashes)
	if err != nil {
		return err
	}
	runner := concurrent.NewState(maxConcurrency)
	var mutex sync.Mutex
	startTime := time.Now()
	for index, foundSize := range foundSizes {
		hashVal := request.Hashes[index]
		if foundSize != 0 {
			if index < len(request.ObjectSizes) &&
				foundSize != request.ObjectSizes[index] {
				mutex.Lock()
				if reply.FailedIndex != 0 {
					reply.FailedIndex = uint(index)
				}
				mutex.Unlock()
				return fmt.Errorf("existing size: %d != %d for: %x",
					foundSize, request.ObjectSizes[index], hashVal)
			}
			continue
		}
		err := runner.GoRun(func() error {
			err := t.importObject(request.BaseRemoteUrl, hashVal, &mutex, reply)
			if err != nil {
				mutex.Lock()
				if reply.FailedIndex != 0 {
					reply.FailedIndex = uint(index)
				}
				mutex.Unlock()
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	if err := runner.Reap(); err != nil {
		return err
	}
	duration := time.Since(startTime)
	speed := float64(reply.BytesAdded) / duration.Seconds()
	t.logger.Printf(
		"ImportObjects() added: %d new objects (%s) in: %s (%s/s)\n",
		reply.ObjectsAdded, format.FormatBytes(reply.BytesAdded),
		format.Duration(duration),
		format.FormatBytes(uint64(speed)))
	return nil
}

func (t *srpcType) importObject(baseUrl string, hashVal hash.Hash,
	mutex *sync.Mutex, reply *objectserver.ImportObjectsResponse) error {
	blobUrl := fmt.Sprintf("%s/%x", baseUrl, hashVal)
	rc, size, err := httpClient.GetReader(blobUrl)
	if err != nil {
		return fmt.Errorf("error getting reader for: %s: %s", blobUrl, err)
	}
	defer rc.Close()
	computedHash, added, err := t.objectServer.AddObject(rc, size, &hashVal)
	if err != nil {
		return err
	}
	if !added {
		t.logger.Printf("imported object already exists: %x\n", hashVal)
	}
	mutex.Lock()
	reply.BytesAdded += size
	reply.ObjectsAdded++
	mutex.Unlock()
	if computedHash != hashVal {
		return fmt.Errorf("claimed hash: %x != computed hash: %",
			hashVal, computedHash)
	}
	return nil
}
