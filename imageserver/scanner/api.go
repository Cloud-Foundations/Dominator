package scanner

import (
	"io"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/lockwatcher"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/objectserver"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/stringutil"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

// TODO: the types should probably be moved into a separate package, leaving
//       behind the scanner code.

const metadataFile = ".metadata"

type Config struct {
	BaseDirectory                       string
	LockCheckInterval                   time.Duration
	LockLogTimeout                      time.Duration
	MaximumExpirationDuration           time.Duration // Default: 1 day.
	MaximumExpirationDurationPrivileged time.Duration // Default: 1 month.
	ReplicationMaster                   string
}

type notifiers map[<-chan string]chan<- string
type makeDirectoryNotifiers map[<-chan image.Directory]chan<- image.Directory

type ImageDataBase struct {
	Config
	Params
	lockWatcher *lockwatcher.LockWatcher
	sync.RWMutex
	// Protected by main lock.
	directoryMap    map[string]image.DirectoryMetadata
	imageMap        map[string]*image.Image
	addNotifiers    notifiers
	deleteNotifiers notifiers
	mkdirNotifiers  makeDirectoryNotifiers
	// Unprotected by main lock.
	deduperLock      sync.Mutex
	deduper          *stringutil.StringDeduplicator
	deduperTrigger   chan<- struct{}
	pendingImageLock sync.Mutex
	objectFetchLock  sync.Mutex
}

type Params struct {
	Logger       log.DebugLogger
	ObjectServer objectserver.FullObjectServer
}

func Load(config Config, params Params) (*ImageDataBase, error) {
	return loadImageDataBase(config, params)
}

func LoadImageDataBase(baseDir string, objSrv objectserver.FullObjectServer,
	replicationMaster string, logger log.DebugLogger) (*ImageDataBase, error) {
	return loadImageDataBase(
		Config{
			BaseDirectory:     baseDir,
			ReplicationMaster: replicationMaster,
		},
		Params{
			Logger:       logger,
			ObjectServer: objSrv,
		})
}

func (imdb *ImageDataBase) AddImage(img *image.Image, name string,
	authInfo *srpc.AuthInformation) error {
	return imdb.addImage(img, name, authInfo)
}

func (imdb *ImageDataBase) ChangeImageExpiration(name string,
	expiresAt time.Time, authInfo *srpc.AuthInformation) (bool, error) {
	return imdb.changeImageExpiration(name, expiresAt, authInfo)
}

func (imdb *ImageDataBase) CheckDirectory(name string) bool {
	return imdb.checkDirectory(name)
}

func (imdb *ImageDataBase) CheckImage(name string) bool {
	return imdb.checkImage(name)
}

func (imdb *ImageDataBase) ChownDirectory(dirname, ownerGroup string,
	authInfo *srpc.AuthInformation) error {
	return imdb.chownDirectory(dirname, ownerGroup, authInfo)
}

func (imdb *ImageDataBase) CountDirectories() uint {
	return imdb.countDirectories()
}

func (imdb *ImageDataBase) CountImages() uint {
	return imdb.countImages()
}

func (imdb *ImageDataBase) DeleteImage(name string,
	authInfo *srpc.AuthInformation) error {
	return imdb.deleteImage(name, authInfo)
}

// DeleteUnreferencedObjects will delete some or all unreferenced objects.
// Objects are randomly selected for deletion, until both the percentage and
// bytes thresholds are satisfied.
// If an image upload/replication is in process this operation is unsafe as it
// may delete objects that the new image will be using.
func (imdb *ImageDataBase) DeleteUnreferencedObjects(percentage uint8,
	bytes uint64) error {
	_, _, err := imdb.Params.ObjectServer.DeleteUnreferenced(percentage, bytes)
	return err
}

func (imdb *ImageDataBase) DoWithPendingImage(img *image.Image,
	doFunc func() error) error {
	return imdb.doWithPendingImage(img, doFunc)
}

func (imdb *ImageDataBase) FindLatestImage(
	request proto.FindLatestImageRequest) (string, error) {
	return imdb.findLatestImage(request)
}

func (imdb *ImageDataBase) GetImage(name string) *image.Image {
	return imdb.getImage(name)
}

func (imdb *ImageDataBase) GetUnreferencedObjectsStatistics() (uint64, uint64) {
	return 0, 0
}

func (imdb *ImageDataBase) ListDirectories() []image.Directory {
	return imdb.listDirectories()
}

func (imdb *ImageDataBase) ListImages() []string {
	return imdb.listImages(proto.ListSelectedImagesRequest{})
}

func (imdb *ImageDataBase) ListSelectedImages(
	request proto.ListSelectedImagesRequest) []string {
	return imdb.listImages(request)
}

// ListUnreferencedObjects will return a map listing all the objects and their
// corresponding sizes which are not referenced by an image.
// Note that some objects may have been recently added and the referencing image
// may not yet be present (i.e. it may be added after missing objects are
// uploaded).
func (imdb *ImageDataBase) ListUnreferencedObjects() map[hash.Hash]uint64 {
	return imdb.Params.ObjectServer.ListUnreferenced()
}

func (imdb *ImageDataBase) MakeDirectory(dirname string,
	authInfo *srpc.AuthInformation) error {
	return imdb.makeDirectory(image.Directory{Name: dirname}, authInfo, true)
}

func (imdb *ImageDataBase) ObjectServer() objectserver.ObjectServer {
	return imdb.Params.ObjectServer
}

func (imdb *ImageDataBase) RegisterAddNotifier() <-chan string {
	return imdb.registerAddNotifier()
}

func (imdb *ImageDataBase) RegisterDeleteNotifier() <-chan string {
	return imdb.registerDeleteNotifier()
}

func (imdb *ImageDataBase) RegisterMakeDirectoryNotifier() <-chan image.Directory {
	return imdb.registerMakeDirectoryNotifier()
}

func (imdb *ImageDataBase) UnregisterAddNotifier(channel <-chan string) {
	imdb.unregisterAddNotifier(channel)
}

func (imdb *ImageDataBase) UnregisterDeleteNotifier(channel <-chan string) {
	imdb.unregisterDeleteNotifier(channel)
}

func (imdb *ImageDataBase) UnregisterMakeDirectoryNotifier(
	channel <-chan image.Directory) {
	imdb.unregisterMakeDirectoryNotifier(channel)
}

func (imdb *ImageDataBase) UpdateDirectory(directory image.Directory) error {
	return imdb.makeDirectory(directory, nil, false)
}

func (imdb *ImageDataBase) WriteHtml(writer io.Writer) {
	imdb.writeHtml(writer)
}
