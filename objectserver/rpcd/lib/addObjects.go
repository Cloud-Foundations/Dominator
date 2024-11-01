package lib

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

type hashValueType hash.Hash

type objectsMapType map[hash.Hash]struct{}

func addObjects(conn *srpc.Conn, decoder srpc.Decoder, encoder srpc.Encoder,
	adder ObjectAdder, logger log.Logger) error {
	defer conn.Flush()
	logger.Printf("AddObjects(%s) starting\n", conn.RemoteAddr())
	// If possible increment the reference count on all objects that are added
	// and then decrement the reference counts shortly after all the objects are
	// added. This prevents a garbage collector from deleting objects just added
	// before they are all added or able to be referenced by an image.
	delayRefcountDecrement := true
	refcountedObjects := make(objectsMapType)
	refcounter, haveRefcounter := adder.(objectserver.ObjectsRefcounter)
	if haveRefcounter {
		defer func() {
			go func() {
				if delayRefcountDecrement {
					time.Sleep(11 * time.Second)
				}
				refcounter.AdjustRefcounts(false, refcountedObjects)
			}()
		}()
	}
	numAdded := 0
	numObj := 0
	startTime := time.Now()
	var bytesAdded, bytesReceived uint64
	for {
		var request proto.AddObjectRequest
		var response proto.AddObjectResponse
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("error decoding after %d objects: %s",
				numObj, err)
		}
		if request.Length < 1 {
			break
		}
		var err error
		response.Hash, response.Added, err =
			adder.AddObject(conn, request.Length, request.ExpectedHash)
		if err == nil && haveRefcounter {
			err = refcountObject(refcounter, refcountedObjects, response.Hash)
		}
		response.ErrorString = errors.ErrorToString(err)
		if err := encoder.Encode(response); err != nil {
			return errors.New("error encoding: " + err.Error())
		}
		numObj++
		if response.ErrorString != "" {
			delayRefcountDecrement = false
			logger.Printf(
				"AddObjects(): failed, %d of %d so far are new objects: %s",
				numAdded, numObj+1, response.ErrorString)
			if err := conn.Flush(); err != nil { // Report error quickly.
				return err
			}
			continue
		}
		bytesReceived += request.Length
		if response.Added {
			bytesAdded += request.Length
			numAdded++
		}
	}
	duration := time.Since(startTime)
	speed := uint64(float64(bytesReceived) / duration.Seconds())
	logger.Printf(
		"AddObjects(): %d (%s) of %d (%s) in %s (%s/s) are new objects",
		numAdded, format.FormatBytes(bytesAdded),
		numObj, format.FormatBytes(bytesReceived),
		format.Duration(duration), format.FormatBytes(speed))
	return nil
}

func refcountObject(refcounter objectserver.ObjectsRefcounter,
	refcountedObjects objectsMapType, hashVal hash.Hash) error {
	if _, ok := refcountedObjects[hashVal]; ok {
		return nil
	}
	hv := hashValueType(hashVal)
	if err := refcounter.AdjustRefcounts(true, hv); err != nil {
		return err
	}
	refcountedObjects[hashVal] = struct{}{}
	return nil
}

func (hv hashValueType) ForEachObject(objectFunc func(hash.Hash) error) error {
	return objectFunc(hash.Hash(hv))
}

func (oi objectsMapType) ForEachObject(objectFunc func(hash.Hash) error) error {
	for hashVal := range oi {
		if err := objectFunc(hashVal); err != nil {
			return err
		}
	}
	return nil
}
