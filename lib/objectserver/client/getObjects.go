package client

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

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
	conn.Encode(request)
	conn.Flush()
	var objectsReader ObjectsReader
	objectsReader.client = objClient
	objectsReader.reader = conn
	err = conn.Decode(&reply)
	if err != nil {
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
	return size,
		ioutil.NopCloser(&io.LimitedReader{R: or.reader, N: int64(size)}), nil
}
