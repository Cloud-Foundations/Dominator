package client

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

func AddDevice(client *srpc.Client, deviceId string, adder func() error) error {
	return addDevice(client, deviceId, adder)
}

func ClaimDevice(client *srpc.Client, deviceId, deviceName string) error {
	return claimDevice(client, deviceId, deviceName)
}

func AssociateStreamWithDevice(srpcClient *srpc.Client, streamName string,
	deviceId string) error {
	return associateStreamWithDevice(srpcClient, streamName, deviceId)
}

func ExportImage(srpcClient *srpc.Client, streamName,
	exportType, exportDestination string) error {
	return exportImage(srpcClient, streamName, exportType, exportDestination)
}

func GetRaw(srpcClient *srpc.Client, streamName string) (
	io.ReadCloser, uint64, error) {
	return getRaw(srpcClient, streamName)
}

func GetStatus(srpcClient *srpc.Client) (proto.GetStatusResponse, error) {
	return getStatus(srpcClient)
}

func PrepareForCapture(srpcClient *srpc.Client, streamName string) error {
	return prepareForCapture(srpcClient, streamName)
}

func PrepareForCopy(srpcClient *srpc.Client, streamName string) error {
	return prepareForCopy(srpcClient, streamName)
}

func PrepareForUnpack(srpcClient *srpc.Client, streamName string,
	skipIfPrepared bool, doNotWaitForResult bool) error {
	return prepareForUnpack(srpcClient, streamName, skipIfPrepared,
		doNotWaitForResult)
}

func RemoveDevice(client *srpc.Client, deviceId string) error {
	return removeDevice(client, deviceId)
}

func UnpackImage(srpcClient *srpc.Client, streamName,
	imageLeafName string) error {
	return unpackImage(srpcClient, streamName, imageLeafName)
}
