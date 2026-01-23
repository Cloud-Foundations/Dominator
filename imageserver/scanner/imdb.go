package scanner

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/filesystem"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
	"github.com/Cloud-Foundations/Dominator/lib/image"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
	"github.com/Cloud-Foundations/Dominator/lib/tags/tagmatcher"
	proto "github.com/Cloud-Foundations/Dominator/proto/imageserver"
)

var (
	errNoAccess   = errors.New("no access to image")
	errNoAuthInfo = errors.New("no authentication information")
)

// writeImage will write an image to the specified filename, ensuring that a
// failure during the process will not leave a corrupted/truncated file.
// The file checksum is returned.
// If exclusive is true, an error will be returned if the file already exists.
func writeImage(filename string, img *image.Image, exclusive bool) (
	[]byte, error) {
	tmpFilename := filename + "~"
	os.Remove(tmpFilename)
	file, err := os.OpenFile(tmpFilename, os.O_CREATE|os.O_RDWR|os.O_EXCL,
		fsutil.PublicFilePerms)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFilename)
	defer file.Close()
	w := bufio.NewWriter(file)
	writer := fsutil.NewChecksumWriter(w)
	encoder := gob.NewEncoder(writer)
	if err := encoder.Encode(img); err != nil {
		return nil, err
	}
	if err := writer.WriteChecksum(); err != nil {
		return nil, err
	}
	fileChecksum := writer.GetChecksum()
	if err := w.Flush(); err != nil {
		return nil, err
	}
	if err := fsutil.FsyncFile(file); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	if exclusive {
		if err := os.Link(tmpFilename, filename); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(tmpFilename, filename); err != nil {
		return nil, err
	}
	return fileChecksum, nil
}

func (imdb *ImageDataBase) addImage(img *image.Image, name string,
	authInfo *srpc.AuthInformation) error {
	if err := img.Verify(); err != nil {
		return err
	}
	if imageIsExpired(img) {
		imdb.Logger.Printf("Ignoring already expired image: %s\n", name)
		return nil
	}
	if err := imdb.prepareToWrite(name); err != nil {
		return err
	}
	doCleanup := true
	defer func() {
		if doCleanup {
			imdb.Lock()
			delete(imdb.imageMap, name)
			imdb.Unlock()
		}
	}()
	if err := imdb.checkPermissions(name, nil, authInfo); err != nil {
		return err
	}
	exclusive := imdb.ReplicationMaster == ""
	if err := imdb.writeImage(name, img, exclusive); err != nil {
		if os.IsExist(err) {
			return errors.New("cannot add previously deleted image: " + name)
		}
		return err
	}
	doCleanup = false
	return nil
}

// changeImageExpiration returns true if the image was changed, else false.
func (imdb *ImageDataBase) changeImageExpiration(name string,
	expiresAt time.Time, authInfo *srpc.AuthInformation) (bool, error) {
	if err := imdb.checkExpiration(expiresAt, authInfo); err != nil {
		return false, err
	}
	imdb.Lock()
	haveLock := true
	defer func() {
		if haveLock {
			imdb.Unlock()
		}
	}()
	imgType, _ := imdb.getImageTypeWithLock(name)
	if imgType == nil {
		return false, errors.New("image not found")
	}
	img := imgType.image
	if img == nil {
		return false, errors.New("image not found")
	}
	if err := imdb.checkPermissions(name, img, authInfo); err != nil {
		return false, err
	}
	if img.ExpiresAt.IsZero() {
		return false, errors.New("image does not expire")
	}
	if imgType.modifying {
		return false, errors.New("image being modified")
	}
	imgType.modifying = true
	defer func() {
		if !haveLock {
			imdb.Lock()
		}
		imgType.modifying = false
		if !haveLock {
			imdb.Unlock()
		}
	}()
	imdb.Unlock()
	haveLock = false
	if expiresAt.IsZero() || expiresAt.After(img.ExpiresAt) {
		fileChecksum, err := imdb.writeNewExpiration(name, img, expiresAt)
		if err != nil {
			return false, err
		}
		imdb.Lock()
		haveLock = true
		img.ExpiresAt = expiresAt
		imgType.fileChecksum = fileChecksum
		imdb.addNotifiers.sendPlain(name, "add", imdb.Logger)
		return true, nil
	}
	if expiresAt.Before(img.ExpiresAt) {
		return false, errors.New("cannot shorten expiration time")
	}
	return false, nil
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
		if authInfo.HaveMethodAccess ||
			authInfo.Username == "" { // Internal call.
			return nil
		}
		return errors.New("not permitted to make image non-expiring")
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

// checkImage returns true if the image exists.
func (imdb *ImageDataBase) checkImage(name string) bool {
	imdb.RLock()
	defer imdb.RUnlock()
	return imdb.imageMap[name] != nil
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

// prepareToWrite returns an error if the image already exists or is being
// written, otherwise it marks the image as being written and returns nil.
// The write lock is grabbed and released.
func (imdb *ImageDataBase) prepareToWrite(name string) error {
	imdb.Lock()
	defer imdb.Unlock()
	if img, ok := imdb.imageMap[name]; ok {
		if img == nil {
			return errors.New("image: " + name + " already being written")
		}
		return errors.New("image: " + name + " already exists")
	}
	imdb.imageMap[name] = nil
	return nil
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
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR,
		fsutil.PublicFilePerms)
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
	if img, ok := imdb.getImageWithLock(name); !ok {
		return errors.New("image: " + name + " does not exist")
	} else if img == nil {
		return errors.New("image: " + name + " is being written")
	} else {
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
	}
}

// This must be called with the main lock held.
func (imdb *ImageDataBase) deleteImageAndUpdateUnreferencedObjectsList(
	name string) {
	img, _ := imdb.getImageWithLock(name)
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
		if img == nil {
			continue
		}
		if request.IgnoreExpiringImages && !img.image.ExpiresAt.IsZero() {
			continue
		}
		// First filter out images we don't want.
		if filepath.Dir(name) != request.DirectoryName {
			continue
		}
		if request.BuildCommitId != "" &&
			request.BuildCommitId != img.image.BuildCommitId &&
			request.BuildCommitId != img.image.BuildBranch {
			continue
		}
		if !tagMatcher.MatchEach(img.image.Tags) {
			continue
		}
		// Select newer image after filtering.
		if img.image.CreatedOn.After(previousCreateTime) {
			imageName = name
			previousCreateTime = img.image.CreatedOn
		}
	}
	return imageName, nil
}

func (imdb *ImageDataBase) getImage(name string) *image.Image {
	imdb.RLock()
	defer imdb.RUnlock()
	img, _ := imdb.getImageWithLock(name)
	return img
}

func (imdb *ImageDataBase) getImageArchive(name string) ([]byte, error) {
	img := imdb.getImage(name)
	if img == nil {
		return nil, fmt.Errorf("image: %s does not exist", name)
	}
	secret, err := imdb.getSecret()
	if err != nil {
		return nil, err
	}
	archiveData := &bytes.Buffer{}
	mac := hmac.New(sha512.New, secret)
	writer := io.MultiWriter(archiveData, mac)
	err = gob.NewEncoder(writer).Encode(proto.ImageArchive{
		ImageName: name,
		Image:     *img,
	})
	if err != nil {
		return nil, err
	}
	if _, err := archiveData.Write(mac.Sum(nil)); err != nil {
		return nil, err
	}
	return archiveData.Bytes(), nil
}

func (imdb *ImageDataBase) getImageFileChecksum(name string) []byte {
	imdb.RLock()
	defer imdb.RUnlock()
	img, ok := imdb.getImageTypeWithLock(name)
	if ok && img != nil {
		return img.fileChecksum
	}
	return nil
}

func (imdb *ImageDataBase) getImageTypeWithLock(name string) (
	*imageType, bool) {
	img, ok := imdb.imageMap[name]
	return img, ok
}

func (imdb *ImageDataBase) getImageWithLock(name string) (*image.Image, bool) {
	img, ok := imdb.getImageTypeWithLock(name)
	if img != nil {
		return img.image, ok
	}
	return nil, ok
}

func (imdb *ImageDataBase) getImageComputedFiles(name string) (
	[]filesystem.ComputedFile, bool) {
	imdb.RLock()
	defer imdb.RUnlock()
	img, _ := imdb.imageMap[name]
	if img == nil {
		return nil, false
	}
	return img.computedFiles, true
}

func (imdb *ImageDataBase) getImageUsageEstimate(name string) (uint64, bool) {
	imdb.RLock()
	defer imdb.RUnlock()
	img, _ := imdb.imageMap[name]
	if img == nil {
		return 0, false
	}
	return img.usageEstimate, true
}

func (imdb *ImageDataBase) getSecret() ([]byte, error) {
	imdb.secretLock.Lock()
	defer imdb.secretLock.Unlock()
	if len(imdb.secret) > 0 {
		return imdb.secret, nil
	}
	filename := filepath.Join(imdb.BaseDirectory, ".secret")
	secret, err := os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(secret) > 0 {
		imdb.secret = secret
		return imdb.secret, nil
	}
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	err = os.WriteFile(filename, secret, fsutil.PrivateFilePerms)
	if err != nil {
		return nil, err
	}
	imdb.secret = secret
	return imdb.secret, nil
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
		if img == nil {
			continue
		}
		if request.IgnoreExpiringImages && !img.image.ExpiresAt.IsZero() {
			continue
		}
		if !tagMatcher.MatchEach(img.image.Tags) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func (imdb *ImageDataBase) makeDirectory(directory image.Directory,
	authInfo *srpc.AuthInformation, userRpc bool) error {
	imdb.Lock()
	defer imdb.Unlock()
	return imdb.makeDirectoryWithLock(directory, authInfo, userRpc)
}

func (imdb *ImageDataBase) makeDirectoryAll(dirname string,
	authInfo *srpc.AuthInformation) error {
	if dirname == "." {
		return errors.New("cannot create root directory")
	}
	imdb.Lock()
	defer imdb.Unlock()
	return imdb.makeDirectoryAllRecurse(dirname, authInfo)
}

func (imdb *ImageDataBase) makeDirectoryAllRecurse(dirname string,
	authInfo *srpc.AuthInformation) error {
	if dirname == "." {
		return nil
	}
	if _, ok := imdb.directoryMap[dirname]; ok {
		return nil
	}
	parentDir := path.Dir(dirname)
	if err := imdb.makeDirectoryAllRecurse(parentDir, authInfo); err != nil {
		return err
	}
	return imdb.makeDirectoryWithLock(image.Directory{Name: dirname}, authInfo,
		true)
}

func (imdb *ImageDataBase) makeDirectoryWithLock(directory image.Directory,
	authInfo *srpc.AuthInformation, userRpc bool) error {
	directory.Name = filepath.Clean(directory.Name)
	pathname := filepath.Join(imdb.BaseDirectory, directory.Name)
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
	if e := os.Mkdir(pathname, fsutil.DirPerms); e != nil && !os.IsExist(e) {
		return e
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

func (imdb *ImageDataBase) restoreImageFromArchive(
	req proto.RestoreImageFromArchiveRequest,
	authInfo *srpc.AuthInformation) error {
	secret, err := imdb.getSecret()
	if err != nil {
		return err
	}
	archive := bytes.NewReader(req.ArchiveData)
	mac := hmac.New(sha512.New, secret)
	var imageArchive proto.ImageArchive
	err = gob.NewDecoder(io.TeeReader(archive, mac)).Decode(&imageArchive)
	if err != nil {
		return err
	}
	img := &imageArchive.Image
	if err := img.Verify(); err != nil {
		return err
	}
	if err := img.VerifyObjects(imdb.Params.ObjectServer); err != nil {
		return err
	}
	if !img.ExpiresAt.IsZero() {
		if waitTime := time.Until(img.ExpiresAt); waitTime >= 0 {
			return fmt.Errorf("cannot restore for: %s",
				format.Duration(waitTime))
		}
		if err := imdb.checkExpiration(req.ExpiresAt, authInfo); err != nil {
			return err
		}
		img.ExpiresAt = req.ExpiresAt
	}
	if err := imdb.prepareToWrite(imageArchive.ImageName); err != nil {
		return err
	}
	doCleanup := true
	defer func() {
		if doCleanup {
			imdb.Lock()
			delete(imdb.imageMap, imageArchive.ImageName)
			imdb.Unlock()
		}
	}()
	providedMac, err := io.ReadAll(archive)
	if err != nil {
		return err
	}
	expectedMac := mac.Sum(nil)
	checksumMatches := hmac.Equal(providedMac, expectedMac)
	if !checksumMatches {
		img.CreatedBy = authInfo.Username
		img.CreatedOn = time.Now()
	}
	err = imdb.writeImage(imageArchive.ImageName, img, !checksumMatches)
	if err != nil {
		if !checksumMatches && os.IsExist(err) {
			return fmt.Errorf("HMAC mismatch: expected: %x, provided: %x",
				expectedMac, providedMac)
		}
		return err
	}
	doCleanup = false
	if authInfo == nil || authInfo.Username == "" {
		imdb.Logger.Printf("RestoreImageFromArchive(%s)\n",
			imageArchive.ImageName)
	} else {
		imdb.Logger.Printf("RestoreImageFromArchive(%s) by %s\n",
			imageArchive.ImageName, authInfo.Username)
	}
	return nil
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

// Write the specified image, assuming other writers are blocked and validation
// checks have been performed.
func (imdb *ImageDataBase) writeImage(name string, img *image.Image,
	exclusive bool) error {
	computedFiles := img.FileSystem.GetComputedFiles()
	usageEstimate := img.FileSystem.EstimateUsage(0)
	filename := filepath.Join(imdb.BaseDirectory, name)
	fileChecksum, err := writeImage(filename, img, exclusive)
	if err != nil {
		return err
	}
	imdb.scheduleExpiration(img, name)
	imdb.Lock()
	imdb.imageMap[name] = &imageType{
		computedFiles: computedFiles,
		fileChecksum:  fileChecksum,
		image:         img,
		usageEstimate: usageEstimate,
	}
	imdb.addNotifiers.sendPlain(name, "add", imdb.Logger)
	imdb.Unlock()
	return imdb.Params.ObjectServer.AdjustRefcounts(true, img)
}

// This must be called with the modifying flag set to true.
// The file checksum and an error are returned.
func (imdb *ImageDataBase) writeNewExpiration(name string,
	oldImage *image.Image, expiresAt time.Time) ([]byte, error) {
	img := *oldImage
	img.ExpiresAt = expiresAt
	filename := filepath.Join(imdb.BaseDirectory, name)
	return writeImage(filename, &img, false)
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
