package unpacker

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func (u *Unpacker) getRaw(streamName string) (io.ReadCloser, uint64, error) {
	u.updateUsageTime()
	defer u.updateUsageTime()
	streamInfo := u.getStream(streamName)
	if streamInfo == nil {
		return nil, 0, errors.New("unknown stream")
	}
	errorChannel := make(chan error, 1)
	readerChannel := make(chan *sizedReader, 1)
	request := requestType{
		request:       requestGetRaw,
		errorChannel:  errorChannel,
		readerChannel: readerChannel,
	}
	streamInfo.requestChannel <- request
	select {
	case reader := <-readerChannel:
		return reader, reader.size, nil
	case err := <-errorChannel:
		if err != nil {
			return nil, 0, err
		}
		return nil, 0, errors.New("no reader")
	}
}

func (stream *streamManagerState) getRaw(
	readerChannel chan<- *sizedReader) error {
	if err := stream.prepareForCopy(); err != nil {
		return err
	}
	streamInfo := stream.streamInfo
	stream.unpacker.rwMutex.RLock()
	device := stream.unpacker.pState.Devices[streamInfo.DeviceId]
	stream.unpacker.rwMutex.RUnlock()
	deviceNode := filepath.Join("/dev", device.DeviceName)
	file, err := os.Open(deviceNode)
	if err != nil {
		return err
	}
	defer file.Close()
	closeNotifier := make(chan struct{}, 1)
	streamInfo.status = proto.StatusStreamTransferring
	reader := &sizedReader{
		closeNotifier: closeNotifier,
		reader:        file,
		size:          device.size,
	}
	readerChannel <- reader
	startTime := time.Now()
	<-closeNotifier
	timeTaken := time.Since(startTime)
	streamInfo.dualLogger.Printf("Transferred(%s) %s in %s (%s/s)\n",
		stream.streamName, format.FormatBytes(reader.nRead),
		format.Duration(timeTaken),
		format.FormatBytes(uint64(float64(reader.nRead)/timeTaken.Seconds())))
	streamInfo.status = proto.StatusStreamNotMounted
	return nil
}

func (rc *sizedReader) Close() error {
	if rc.closeNotifier != nil {
		rc.closeNotifier <- struct{}{}
	}
	return rc.err
}

func (rc *sizedReader) Read(p []byte) (int, error) {
	if rc.err != nil {
		return 0, rc.err
	}
	nRead, err := rc.reader.Read(p)
	rc.nRead += uint64(nRead)
	rc.err = err
	if err != nil {
		rc.closeNotifier <- struct{}{}
		rc.closeNotifier = nil
	}
	return nRead, err
}
