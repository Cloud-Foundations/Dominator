package main

import (
	"fmt"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

func importObjectsSubcommand(args []string, logger log.DebugLogger) error {
	err := importObjects(fmt.Sprintf("%s:%d",
		*objectServerHostname, *objectServerPortNum), args[0], args[1], logger)
	if err != nil {
		return fmt.Errorf("error importing objects: %s", err)
	}
	return nil
}

func importObjects(address, baseUrl, hashesFilename string,
	logger log.DebugLogger) error {
	hashes, err := loadHashes(hashesFilename)
	if err != nil {
		return err
	}
	client, err := srpc.DialHTTP("tcp", address, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	request := proto.ImportObjectsRequest{
		BaseRemoteUrl: baseUrl,
		Hashes:        hashes,
	}
	var response proto.ImportObjectsResponse
	startTime := time.Now()
	err = client.RequestReply("ObjectServer.ImportObjects", request, &response)
	if err != nil {
		return err
	}
	if err := errors.New(response.Error); err != nil {
		return err
	}
	duration := time.Since(startTime)
	speed := float64(response.BytesAdded) / duration.Seconds()
	logger.Printf("Imported %d new objects (%s) in %s (%s/s)\n",
		response.ObjectsAdded, format.FormatBytes(response.BytesAdded),
		format.Duration(duration), format.FormatBytes(uint64(speed)))
	return nil
}
