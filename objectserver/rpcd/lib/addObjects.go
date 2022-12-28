package lib

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/proto/objectserver"
)

func addObjects(conn *srpc.Conn, decoder srpc.Decoder, encoder srpc.Encoder,
	adder ObjectAdder, logger log.Logger) error {
	defer conn.Flush()
	logger.Printf("AddObjects(%s) starting\n", conn.RemoteAddr())
	numAdded := 0
	numObj := 0
	startTime := time.Now()
	var bytesAdded, bytesReceived uint64
	for {
		var request objectserver.AddObjectRequest
		var response objectserver.AddObjectResponse
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
		response.ErrorString = errors.ErrorToString(err)
		if err := encoder.Encode(response); err != nil {
			return errors.New("error encoding: " + err.Error())
		}
		numObj++
		if response.ErrorString != "" {
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
