package client

import (
	"errors"
	"fmt"
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

type objectReader struct {
	*io.LimitedReader
}

func (objClient *ObjectClient) getObjects(hashes []hash.Hash) (
	*ObjectsReader, error) {
	client, err := objClient.getClient()
	if err != nil {
		return nil, err
	}
	conn, err := client.Call("ObjectServer.GetObjects")
	if err != nil {
		return nil, fmt.Errorf("error calling: %s\n", err)
	}
	var request objectserver.GetObjectsRequest
	var reply objectserver.GetObjectsResponse
	request.Exclusive = objClient.exclusiveGet
	request.Hashes = hashes
	if err := conn.Encode(request); err != nil {
		return nil, err
	}
	if err := conn.Flush(); err != nil {
		return nil, err
	}
	var objectsReader ObjectsReader
	objectsReader.client = objClient
	objectsReader.reader = conn
	objectsReader.sharedOR = &objectReader{
		LimitedReader: &io.LimitedReader{R: conn},
	}
	if err := conn.Decode(&reply); err != nil {
		return nil, err
	}
	if reply.ResponseString != "" {
		return nil, errors.New(reply.ResponseString)
	}
	objectsReader.nextIndex = -1
	objectsReader.sizes = reply.ObjectSizes
	return &objectsReader, nil
}

func (or *ObjectsReader) close() error {
	return or.reader.Close()
}

func (or *ObjectsReader) nextObject() (uint64, io.ReadCloser, error) {
	or.nextIndex++
	if or.nextIndex >= int64(len(or.sizes)) {
		return 0, nil, errors.New("all objects have been consumed")
	}
	size := or.sizes[or.nextIndex]
	or.sharedOR.N = int64(size)
	return size, or.sharedOR, nil
}

func (r *objectReader) Close() error {
	return nil
}

func (r *objectReader) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, r.LimitedReader)
}
