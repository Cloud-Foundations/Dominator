package scanner

import (
	"bufio"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

const (
	dirPerms = syscall.S_IRWXU | syscall.S_IRGRP | syscall.S_IXGRP |
		syscall.S_IROTH | syscall.S_IXOTH
	filePerms = syscall.S_IRUSR | syscall.S_IWUSR | syscall.S_IRGRP |
		syscall.S_IROTH
)

var (
	errNoAccess   = errors.New("no access to image")
	errNoAuthInfo = errors.New("no authentication information")
)

func (imdb *ImageDataBase) addImage(img *image.Image, name string,
	authInfo *srpc.AuthInformation) error {
	if err := img.Verify(); err != nil {
		return err
	}
	if imageIsExpired(img) {
		imdb.Logger.Printf("Ignoring already expired image: %s\n", name)
		return nil
	}
	imdb.Lock()
	doUnlock := true
	defer func() {
		if doUnlock {
			imdb.Unlock()
		}
	}()
	if _, ok := imdb.imageMap[name]; ok {
		return errors.New("image: " + name + " already exists")
	} else {
		if err := imdb.checkPermissions(name, nil, authInfo); err != nil {
			return err
		}
		filename := filepath.Join(imdb.BaseDirectory, name)
		flags := os.O_CREATE | os.O_RDWR
		if imdb.ReplicationMaster == "" {
			flags |= os.O_EXCL // I am the master.
		} else {
			flags |= os.O_TRUNC
		}
		file, err := os.OpenFile(filename, flags, filePerms)
		if err != nil {
			if os.IsExist(err) {
				return errors.New("cannot add previously deleted image: " +
					name)
			}
			return err
		}
		defer file.Close()
		w := bufio.NewWriter(file)
		defer w.Flush()
		writer := fsutil.NewChecksumWriter(w)
		defer writer.WriteChecksum()
		encoder := gob.NewEncoder(writer)
		if err := encoder.Encode(img); err != nil {
			os.Remove(filename)
			return err
		}
		if err := w.Flush(); err != nil {
			os.Remove(filename)
			return err
		}
		imdb.scheduleExpiration(img, name)
		imdb.imageMap[name] = img
		imdb.addNotifiers.sendPlain(name, "add", imdb.Logger)
		doUnlock = false
		imdb.Unlock()
		return imdb.Params.ObjectServer.AdjustRefcounts(true, img)
	}
}

func (imdb *ImageDataBase) changeImageExpiration(name string,
	expiresAt time.Time, authInfo *srpc.AuthInformation) (bool, error) {
	if err := imdb.checkExpiration(expiresAt, authInfo); err != nil {
		return false, err
	}
	imdb.Lock()
	defer imdb.Unlock()
	if img, ok := imdb.imageMap[name]; !ok {
		return false, errors.New("image not found")
	} else if err := imdb.checkPermissions(name, img, authInfo); err != nil {
		return false, err
	} else if img.ExpiresAt.IsZero() {
		return false, errors.New("image does not expire")
	} else if expiresAt.IsZero() {
		if err := imdb.writeNewExpiration(name, img, expiresAt); err != nil {
			return false, err
		}
		img.ExpiresAt = expiresAt
		imdb.addNotifiers.sendPlain(name, "add", imdb.Logger)
		return true, nil
	} else if expiresAt.Before(img.ExpiresAt) {
		return false, errors.New("cannot shorten expiration time")
	} else if expiresAt.After(img.ExpiresAt) {
		if err := imdb.writeNewExpiration(name, img, expiresAt); err != nil {
			return false, err
		}
		img.ExpiresAt = expiresAt
		imdb.addNotifiers.sendPlain(name, "add", imdb.Logger)
		return true, nil
	} else {
		return false, nil
	}
}

// This must be called with the lock held.
func (imdb *ImageDataBase) checkChown(dirname, ownerGroup string,
	authInfo *srpc.AuthInformation) error {
	if authInfo == nil {
		return errNoAuthInfo
	}
	if authInfo.HaveMethodAccess {
		return nil
	}
	// If owner of parent, any group can be set.
	parentDirname := filepath.Dir(dirname)
	if directoryMetadata, ok := imdb.directoryMap[parentDirname]; ok {
		if directoryMetadata.OwnerGroup != "" {
			if _, ok := authInfo.GroupList[directoryMetadata.OwnerGroup]; ok {
				return nil
			}
		}
	}
	if _, ok := authInfo.GroupList[ownerGroup]; !ok {
		return fmt.Errorf("no membership of %s group", ownerGroup)
	}
	if directoryMetadata, ok := imdb.directoryMap[dirname]; !ok {
		return fmt.Errorf("no metadata for: \"%s\"", dirname)
	} else if directoryMetadata.OwnerGroup != "" {
		if _, ok := authInfo.GroupList[directoryMetadata.OwnerGroup]; ok {
			return nil
		}
	}
	return errNoAccess
}

func (imdb *ImageDataBase) checkDirectory(name string) bool {
	imdb.RLock()
	defer imdb.RUnlock()
	_, ok := imdb.directoryMap[name]
	return ok
}

func (imdb *ImageDataBase) checkExpiration(expiresAt time.Time,
	authInfo *srpc.AuthInformation) error {
	if expiresAt.IsZero() {
		return nil
	}
	expiresIn := time.Until(expiresAt)
	if authInfo != nil && authInfo.HaveMethodAccess {
		if authInfo.Username == "" {
			return nil // Internal call.
		}
		if expiresIn > imdb.MaximumExpirationDurationPrivileged {
			return fmt.Errorf("maximum expiration time is %s for you",
				format.Duration(imdb.MaximumExpirationDurationPrivileged))
		}
		return nil
	}
	if expiresIn > imdb.MaximumExpirationDuration {
		return fmt.Errorf("maximum expiration time is %s",
			format.Duration(imdb.MaximumExpirationDuration))
	}
	return nil
}

func (imdb *ImageDataBase) checkImage(name string) bool {
	imdb.RLock()
	defer imdb.RUnlock()
	_, ok := imdb.imageMap[name]
	return ok
}

// This must be called with the lock held.
func (imdb *ImageDataBase) checkPermissions(imageName string, img *image.Image,
	authInfo *srpc.AuthInformation) error {
	if authInfo == nil {
		return errNoAuthInfo
	}
	if authInfo.HaveMethodAccess {
		return nil
	}
	if authInfo.Username != "" && img != nil {
		if img.CreatedBy == authInfo.Username ||
			img.CreatedFor == authInfo.Username {
			return nil
		}
	}
	dirname := filepath.Dir(imageName)
	if directoryMetadata, ok := imdb.directoryMap[dirname]; !ok {
		return fmt.Errorf("no metadata for: \"%s\"", dirname)
	} else if directoryMetadata.OwnerGroup != "" {
		if _, ok := authInfo.GroupList[directoryMetadata.OwnerGroup]; ok {
			return nil
		}
	}
	return errNoAccess
}

func (imdb *ImageDataBase) chownDirectory(dirname, ownerGroup string,
	authInfo *srpc.AuthInformation) error {
	dirname = filepath.Clean(dirname)
	imdb.Lock()
	defer imdb.Unlock()
	directoryMetadata, ok := imdb.directoryMap[dirname]
	if !ok {
		return fmt.Errorf("no metadata for: \"%s\"", dirname)
	}
	if err := imdb.checkChown(dirname, ownerGroup, authInfo); err != nil {
		return err
	}
	directoryMetadata.OwnerGroup = ownerGroup
	return imdb.updateDirectoryMetadata(
		image.Directory{Name: dirname, Metadata: directoryMetadata})
}

// This must be called with the lock held.
func (imdb *ImageDataBase) updateDirectoryMetadata(
	directory image.Directory) error {
	oldDirectoryMetadata, ok := imdb.directoryMap[directory.Name]
	if ok && directory.Metadata == oldDirectoryMetadata {
		return nil
	}
	if err := imdb.updateDirectoryMetadataFile(directory); err != nil {
		return err
	}
	imdb.directoryMap[directory.Name] = directory.Metadata
	imdb.mkdirNotifiers.sendMakeDirectory(directory, imdb.Logger)
	return nil
}

func (imdb *ImageDataBase) updateDirectoryMetadataFile(
	directory image.Directory) error {
	filename := filepath.Join(imdb.BaseDirectory, directory.Name, metadataFile)
	_, ok := imdb.directoryMap[directory.Name]
	if directory.Metadata == (image.DirectoryMetadata{}) {
		if !ok {
			return nil
		}
		return os.Remove(filename)
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, filePerms)
	if err != nil {
		return err
	}
	if err := writeDirectoryMetadata(file, directory.Metadata); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func writeDirectoryMetadata(file io.Writer,
	directoryMetadata image.DirectoryMetadata) error {
	w := bufio.NewWriter(file)
	writer := fsutil.NewChecksumWriter(w)
	if err := gob.NewEncoder(writer).Encode(directoryMetadata); err != nil {
		return err
	}
	if err := writer.WriteChecksum(); err != nil {
		return err
	}
	return w.Flush()
}

func (imdb *ImageDataBase) countDirectories() uint {
	imdb.RLock()
	defer imdb.RUnlock()
	return uint(len(imdb.directoryMap))
}

func (imdb *ImageDataBase) countImages() uint {
	imdb.RLock()
	defer imdb.RUnlock()
	return uint(len(imdb.imageMap))
}

func (imdb *ImageDataBase) deleteImage(name string,
	authInfo *srpc.AuthInformation) error {
	imdb.Lock()
	defer imdb.Unlock()
	if img, ok := imdb.imageMap[name]; ok {
		if err := imdb.checkPermissions(name, img, authInfo); err != nil {
			return err
		}
		filename := filepath.Join(imdb.BaseDirectory, name)
		if err := os.Truncate(filename, 0); err != nil {
			return err
		}
		imdb.deleteImageAndUpdateUnreferencedObjectsList(name)
		imdb.deleteNotifiers.sendPlain(name, "delete", imdb.Logger)
		return nil
	} else {
		return errors.New("image: " + name + " does not exist")
	}
}

// This must be called with the main lock held.
func (imdb *ImageDataBase) deleteImageAndUpdateUnreferencedObjectsList(
	name string) {
	img := imdb.imageMap[name]
	if img == nil { // May be nil if expiring an already deleted image.
		return
	}
	delete(imdb.imageMap, name)
	imdb.Params.ObjectServer.AdjustRefcounts(false, img)
}

func (imdb *ImageDataBase) doWithPendingImage(img *image.Image,
	doFunc func() error) error {
	imdb.pendingImageLock.Lock()
	defer imdb.pendingImageLock.Unlock()
	return doFunc()
}

func (imdb *ImageDataBase) findLatestImage(
	request proto.FindLatestImageRequest) (string, error) {
	imdb.RLock()
	defer imdb.RUnlock()
	if _, ok := imdb.directoryMap[request.DirectoryName]; !ok {
		return "", errors.New("unknown directory: " + request.DirectoryName)
	}
	var previousCreateTime time.Time
	var imageName string
	tagMatcher := tagmatcher.New(request.TagsToMatch, false)
	for name, img := range imdb.imageMap {
		if request.IgnoreExpiringImages && !img.ExpiresAt.IsZero() {
			continue
		}
		// First filter out images we don't want.
		if filepath.Dir(name) != request.DirectoryName {
			continue
		}
		if request.BuildCommitId != "" &&
			request.BuildCommitId != img.BuildCommitId {
			continue
		}
		if !tagMatcher.MatchEach(img.Tags) {
			continue
		}
		// Select newer image after filtering.
		if img.CreatedOn.After(previousCreateTime) {
			imageName = name
			previousCreateTime = img.CreatedOn
		}
	}
	return imageName, nil
}

func (imdb *ImageDataBase) getImage(name string) *image.Image {
	imdb.RLock()
	defer imdb.RUnlock()
	return imdb.imageMap[name]
}

func (imdb *ImageDataBase) listDirectories() []image.Directory {
	imdb.RLock()
	defer imdb.RUnlock()
	directories := make([]image.Directory, 0, len(imdb.directoryMap))
	for name, metadata := range imdb.directoryMap {
		directories = append(directories,
			image.Directory{Name: name, Metadata: metadata})
	}
	return directories
}

func (imdb *ImageDataBase) listImages(
	request proto.ListSelectedImagesRequest) []string {
	tagMatcher := tagmatcher.New(request.TagsToMatch, false)
	imdb.RLock()
	defer imdb.RUnlock()
	names := make([]string, 0)
	for name, img := range imdb.imageMap {
		if request.IgnoreExpiringImages && !img.ExpiresAt.IsZero() {
			continue
		}
		if !tagMatcher.MatchEach(img.Tags) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func (imdb *ImageDataBase) makeDirectory(directory image.Directory,
	authInfo *srpc.AuthInformation, userRpc bool) error {
	directory.Name = filepath.Clean(directory.Name)
	pathname := filepath.Join(imdb.BaseDirectory, directory.Name)
	imdb.Lock()
	defer imdb.Unlock()
	oldDirectoryMetadata, ok := imdb.directoryMap[directory.Name]
	if userRpc {
		if authInfo == nil {
			return errNoAuthInfo
		}
		if ok {
			return fmt.Errorf("directory: %s already exists", directory.Name)
		}
		directory.Metadata = oldDirectoryMetadata
		parentMetadata, ok := imdb.directoryMap[filepath.Dir(directory.Name)]
		if !ok {
			return fmt.Errorf("no metadata for: %s",
				filepath.Dir(directory.Name))
		}
		if !authInfo.HaveMethodAccess {
			if parentMetadata.OwnerGroup == "" {
				return errNoAccess
			}
			if _, ok := authInfo.GroupList[parentMetadata.OwnerGroup]; !ok {
				return fmt.Errorf("no membership of %s group",
					parentMetadata.OwnerGroup)
			}
		}
		directory.Metadata.OwnerGroup = parentMetadata.OwnerGroup
	}
	if err := os.Mkdir(pathname, dirPerms); err != nil && !os.IsExist(err) {
		return err
	}
	return imdb.updateDirectoryMetadata(directory)
}

func (imdb *ImageDataBase) registerAddNotifier() <-chan string {
	channel := make(chan string, 1)
	imdb.Lock()
	defer imdb.Unlock()
	imdb.addNotifiers[channel] = channel
	return channel
}

func (imdb *ImageDataBase) registerDeleteNotifier() <-chan string {
	channel := make(chan string, 1)
	imdb.Lock()
	defer imdb.Unlock()
	imdb.deleteNotifiers[channel] = channel
	return channel
}

func (imdb *ImageDataBase) registerMakeDirectoryNotifier() <-chan image.Directory {
	channel := make(chan image.Directory, 1)
	imdb.Lock()
	defer imdb.Unlock()
	imdb.mkdirNotifiers[channel] = channel
	return channel
}

func (imdb *ImageDataBase) unregisterAddNotifier(channel <-chan string) {
	imdb.Lock()
	defer imdb.Unlock()
	delete(imdb.addNotifiers, channel)
}

func (imdb *ImageDataBase) unregisterDeleteNotifier(channel <-chan string) {
	imdb.Lock()
	defer imdb.Unlock()
	delete(imdb.deleteNotifiers, channel)
}

func (imdb *ImageDataBase) unregisterMakeDirectoryNotifier(
	channel <-chan image.Directory) {
	imdb.Lock()
	defer imdb.Unlock()
	delete(imdb.mkdirNotifiers, channel)
}

// This must be called with the lock held.
func (imdb *ImageDataBase) writeNewExpiration(name string,
	oldImage *image.Image, expiresAt time.Time) error {
	img := *oldImage
	img.ExpiresAt = expiresAt
	filename := filepath.Join(imdb.BaseDirectory, name)
	tmpFilename := filename + "~"
	file, err := os.OpenFile(tmpFilename, os.O_CREATE|os.O_RDWR|os.O_EXCL,
		filePerms)
	if err != nil {
		return err
	}
	defer file.Close()
	defer os.Remove(tmpFilename)
	w := bufio.NewWriter(file)
	defer w.Flush()
	writer := fsutil.NewChecksumWriter(w)
	encoder := gob.NewEncoder(writer)
	if err := encoder.Encode(img); err != nil {
		return err
	}
	if err := writer.WriteChecksum(); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fsutil.FsyncFile(file)
	return os.Rename(tmpFilename, filename)
}

func (n notifiers) sendPlain(name string, operation string,
	logger log.Logger) {
	if len(n) < 1 {
		return
	} else {
		plural := "s"
		if len(n) < 2 {
			plural = ""
		}
		logger.Printf("Sending %s notification to: %d listener%s\n",
			operation, len(n), plural)
	}
	for _, sendChannel := range n {
		go func(channel chan<- string) {
			channel <- name
		}(sendChannel)
	}
}

func (n makeDirectoryNotifiers) sendMakeDirectory(dir image.Directory,
	logger log.Logger) {
	if len(n) < 1 {
		return
	} else {
		plural := "s"
		if len(n) < 2 {
			plural = ""
		}
		logger.Printf("Sending mkdir notification to: %d listener%s\n",
			len(n), plural)
	}
	for _, sendChannel := range n {
		go func(channel chan<- image.Directory) {
			channel <- dir
		}(sendChannel)
	}
}
