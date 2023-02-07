package unpacker

import (
	"errors"

	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func (u *Unpacker) forgetStream(streamName string) error {
	u.updateUsageTime()
	defer u.updateUsageTime()
	streamInfo := u.getStream(streamName)
	if streamInfo == nil {
		return errors.New("unknown stream")
	}
	errorChannel := make(chan error)
	request := requestType{
		request:      requestForget,
		errorChannel: errorChannel,
	}
	streamInfo.requestChannel <- request
	return <-errorChannel
}

func (stream *streamManagerState) forget() error {
	streamInfo := stream.streamInfo
	switch streamInfo.status {
	case proto.StatusStreamNoDevice:
	case proto.StatusStreamNoFileSystem:
	default:
		if err := stream.unmount(); err != nil {
			return err
		}
	}
	u := stream.unpacker
	u.rwMutex.Lock()
	defer u.rwMutex.Unlock()
	if device, ok := u.pState.Devices[streamInfo.DeviceId]; ok {
		device.eraseFileSystem()
		device.StreamName = ""
		u.pState.Devices[streamInfo.DeviceId] = device
	}
	streamInfo.DeviceId = ""
	delete(u.pState.ImageStreams, stream.streamName)
	return u.writeStateWithLock()
}
