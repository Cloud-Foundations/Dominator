package logarchiver

import (
	"container/list"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/format"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

type buildLogArchiver struct {
	ageList      list.List                   // Oldest first.
	imageStreams map[string]*imageStreamType // Key: stream name.
	mutex        sync.Mutex
	options      BuildLogArchiveOptions
	params       BuildLogArchiveParams
	totalSize    uint64
}

type imageStreamType struct {
	images map[string]*imageType // Key: image leaf name.
	name   string
}

type imageType struct {
	ageListElement    *list.Element
	error             string
	imageStream       *imageStreamType
	logSize           uint64
	modTime           time.Time
	name              string // Leaf name.
	requestorUsername string
}

func makeEntry(errorString string, logSize uint64,
	modTime time.Time, name string, requestorUsername string) *imageType {
	image := &imageType{
		error:             errorString,
		logSize:           logSize,
		name:              filepath.Base(name),
		requestorUsername: requestorUsername,
	}
	return image
}

// readString will read a string from the file specified by filename. The
// trailing newline is stripped if present.
func readString(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	if len(data) < 1 {
		return "", nil
	}
	if data[len(data)-1] == '\n' {
		return string(data[:len(data)-1]), nil
	}
	return string(data), nil
}

// writeString will write the specified string data and a trailing newline to
// the file specified by filename. If the string is empty, the file is not
// written.
func writeString(filename, data string) error {
	if data == "" {
		return nil
	}
	return ioutil.WriteFile(filename, []byte(data+"\n"), fsutil.PublicFilePerms)
}

func newBuildLogArchive(options BuildLogArchiveOptions,
	params BuildLogArchiveParams) (*buildLogArchiver, error) {
	archive := &buildLogArchiver{
		imageStreams: make(map[string]*imageStreamType),
		options:      options,
		params:       params,
	}
	startTime := time.Now()
	if err := archive.load(""); err != nil {
		return nil, err
	}
	loadedTime := time.Now()
	archive.makeAgeList()
	sortedTime := time.Now()
	archive.params.Logger.Printf(
		"Loaded build log archive %s in %s, sorted in %s\n",
		format.FormatBytes(archive.totalSize),
		format.Duration(loadedTime.Sub(startTime)),
		format.Duration(sortedTime.Sub(loadedTime)))
	return archive, nil
}

func (a *buildLogArchiver) addEntry(image *imageType, name string) {
	streamName := filepath.Dir(name)
	imageStream := a.imageStreams[streamName]
	if imageStream == nil {
		imageStream = &imageStreamType{
			images: make(map[string]*imageType),
			name:   streamName,
		}
		a.imageStreams[streamName] = imageStream
	}
	image.imageStream = imageStream
	imageStream.images[image.name] = image
	a.totalSize += image.totalSize()
}

func (a *buildLogArchiver) addEntryWithCheck(image *imageType,
	name string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	for image.totalSize()+a.totalSize < a.options.Quota {
		a.addEntry(image, name)
		return nil
	}
	targetSize := a.options.Quota * 95 / 100
	if image.totalSize()+targetSize > a.options.Quota {
		targetSize -= image.totalSize()
	}
	var deletedLogs uint
	origTotalSize := a.totalSize
	for a.totalSize > targetSize {
		oldestElement := a.ageList.Front()
		if err := a.deleteEntry(oldestElement); err != nil {
			return err
		}
		a.ageList.Remove(oldestElement)
		deletedLogs++
	}
	a.params.Logger.Printf("Deleted %d archived build logs consuming %s\n",
		deletedLogs, format.FormatBytes(origTotalSize-a.totalSize))
	a.addEntry(image, name)
	return nil
}

func (a *buildLogArchiver) deleteEntry(element *list.Element) error {
	image := element.Value.(*imageType)
	imageStream := image.imageStream
	dirname := filepath.Join(a.options.Topdir, imageStream.name, image.name)
	if err := os.RemoveAll(dirname); err != nil {
		return err
	}
	delete(imageStream.images, image.name)
	a.totalSize -= image.totalSize()
	return nil
}

func (a *buildLogArchiver) AddBuildLog(buildInfo BuildInfo,
	buildLog []byte) error {
	dirname := filepath.Join(a.options.Topdir, buildInfo.ImageName)
	if err := os.MkdirAll(filepath.Dir(dirname), fsutil.DirPerms); err != nil {
		return err
	}
	if err := os.Mkdir(dirname, fsutil.DirPerms); err != nil {
		return err
	}
	doDelete := true
	defer func() {
		if doDelete {
			os.RemoveAll(dirname)
		}
	}()
	errorString := errors.ErrorToString(buildInfo.Error)
	err := writeString(filepath.Join(dirname, "error"), errorString)
	if err != nil {
		return err
	}
	logfile := filepath.Join(dirname, "buildLog")
	err = ioutil.WriteFile(logfile, buildLog, fsutil.PublicFilePerms)
	if err != nil {
		return err
	}
	err = writeString(filepath.Join(dirname, "requestorUsername"),
		buildInfo.RequestorUsername)
	if err != nil {
		return err
	}
	image := makeEntry(errorString, uint64(len(buildLog)), time.Now(),
		buildInfo.ImageName, buildInfo.RequestorUsername)
	if err := a.addEntryWithCheck(image, buildInfo.ImageName); err != nil {
		return err
	}
	doDelete = false
	a.params.Logger.Debugf(0, "Archived build log for: %s, %s (%s total)\n",
		buildInfo.ImageName, format.FormatBytes(image.totalSize()),
		format.FormatBytes(a.totalSize))
	return nil
}

func (a *buildLogArchiver) load(dirname string) error {
	dirpath := filepath.Join(a.options.Topdir, dirname)
	names, err := fsutil.ReadDirnames(dirpath, false)
	if err != nil {
		return err
	}
	var errorPathname, buildLogPathname, requestorUsernamePathname string
	for _, name := range names {
		switch name {
		case "error":
			errorPathname = filepath.Join(dirpath, name)
			continue
		case "buildLog":
			buildLogPathname = filepath.Join(dirpath, name)
			continue
		case "requestorUsername":
			requestorUsernamePathname = filepath.Join(dirpath, name)
			continue
		}
		if err := a.load(filepath.Join(dirname, name)); err != nil {
			return err
		}
	}
	if buildLogPathname == "" {
		return nil
	}
	var errorString, requestorUsername string
	if errorPathname != "" {
		errorString, err = readString(errorPathname)
		if err != nil {
			return err
		}
	}
	if requestorUsernamePathname != "" {
		requestorUsername, err = readString(requestorUsernamePathname)
		if err != nil {
			return err
		}
	}
	if fi, err := os.Stat(buildLogPathname); err != nil {
		return err
	} else {
		image := makeEntry(errorString, uint64(fi.Size()), fi.ModTime(),
			dirname, requestorUsername)
		a.addEntry(image, dirname)
	}
	return nil
}

func (a *buildLogArchiver) makeAgeList() {
	var imageList []*imageType
	for _, imageStream := range a.imageStreams {
		for _, image := range imageStream.images {
			imageList = append(imageList, image)
		}
	}
	// Sort so that oldest mtime is the first slice entry.
	sort.Slice(imageList, func(i, j int) bool {
		return imageList[i].modTime.Before(imageList[j].modTime)
	})
	for _, image := range imageList {
		image.ageListElement = a.ageList.PushBack(image)
	}
}

func (image *imageType) totalSize() uint64 {
	size := image.logSize
	if image.error != "" {
		size += uint64(len(image.error) + 1)
	}
	if image.requestorUsername != "" {
		size += uint64(len(image.requestorUsername) + 1)
	}
	return size
}
