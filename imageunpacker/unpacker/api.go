package unpacker

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectcache"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageunpacker"
)

const (
	requestAssociateWithDevice = iota
	requestScan
	requestUnpack
	requestPrepareForCapture
	requestPrepareForCopy
	requestExport
	requestGetRaw
)

var (
	stateFile = "state.json"
)

type deviceInfo struct {
	DeviceName         string
	partitionTimestamp time.Time
	size               uint64
	StreamName         string
}

type requestType struct {
	request           int
	desiredFS         *filesystem.FileSystem
	imageName         string
	deviceId          string
	skipIfPrepared    bool
	exportType        string
	exportDestination string
	errorChannel      chan<- error
	readerChannel     chan<- *sizedReader
}

type imageStreamInfo struct {
	DeviceId       string
	status         proto.StreamStatus
	requestChannel chan<- requestType
	scannedFS      *filesystem.FileSystem
}

type persistentState struct {
	Devices      map[string]deviceInfo       // Key: DeviceId.
	ImageStreams map[string]*imageStreamInfo // Key: StreamName.
}

type sizedReader struct {
	closeNotifier chan<- struct{}
	err           error
	nRead         uint64
	reader        io.Reader
	size          uint64
}

type streamManagerState struct {
	unpacker    *Unpacker
	streamName  string
	streamInfo  *imageStreamInfo
	fileSystem  *filesystem.FileSystem
	objectCache objectcache.ObjectCache
	rootLabel   string
}

type Unpacker struct {
	baseDir            string
	imageServerAddress string
	logger             log.DebugLogger
	rwMutex            sync.RWMutex // Protect below.
	pState             persistentState
	scannedDevices     map[string]struct{}
	lastUsedTime       time.Time
}

func Load(baseDir string, imageServerAddress string, logger log.DebugLogger) (
	*Unpacker, error) {
	return load(baseDir, imageServerAddress, logger)
}

func (u *Unpacker) AddDevice(deviceId string) error {
	return u.addDevice(deviceId)
}

func (u *Unpacker) ClaimDevice(deviceId, deviceName string) error {
	return u.claimDevice(deviceId, deviceName)
}

func (u *Unpacker) AssociateStreamWithDevice(streamName string,
	deviceId string) error {
	return u.associateStreamWithDevice(streamName, deviceId)
}

func (u *Unpacker) ExportImage(streamName string, exportType string,
	exportDestination string) error {
	return u.exportImage(streamName, exportType, exportDestination)
}

func (u *Unpacker) GetFileSystem(streamName string) (
	*filesystem.FileSystem, error) {
	return u.getFileSystem(streamName)
}

func (u *Unpacker) GetRaw(streamName string) (io.ReadCloser, uint64, error) {
	return u.getRaw(streamName)
}

func (u *Unpacker) GetStatus() proto.GetStatusResponse {
	return u.getStatus()
}

func (u *Unpacker) PrepareForCapture(streamName string) error {
	return u.prepareForCapture(streamName)
}

func (u *Unpacker) PrepareForCopy(streamName string) error {
	return u.prepareForCopy(streamName)
}

func (u *Unpacker) PrepareForUnpack(streamName string, skipIfPrepared bool,
	doNotWaitForResult bool) error {
	return u.prepareForUnpack(streamName, skipIfPrepared, doNotWaitForResult)
}

func (u *Unpacker) PrepareForAddDevice() error {
	return u.prepareForAddDevice()
}

func (u *Unpacker) RemoveDevice(deviceId string) error {
	return u.removeDevice(deviceId)
}

func (u *Unpacker) UnpackImage(streamName string, imageLeafName string) error {
	return u.unpackImage(streamName, imageLeafName)
}

func (u *Unpacker) WriteHtml(writer io.Writer) {
	u.writeHtml(writer)
}
